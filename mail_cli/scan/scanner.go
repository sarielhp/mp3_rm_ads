package scan

import (
	"bytes"
	"fmt"
	"log/slog"
	"mail_cli/app"
	"mail_cli/backend/gmail"
	"mail_cli/cache/msg"
	"mail_cli/cfg_g"
	"mail_cli/email"
	"mime"
	"net/mail"
	"strings"
	"sync"
	"unicode"

	"github.com/abadojack/whatlanggo"
)

func ScanEmails(ids []string, config *cfg_g.Config, cacheSubdir string) ([]string, []string, map[string]*Response, error) {
	var spamIDs []string
	var blacklistedIDs []string
	scanResults := make(map[string]*Response)

	var mu sync.Mutex
	var wg sync.WaitGroup

	sem := make(chan struct{}, 8)

	var scanErr error
	dec := new(mime.WordDecoder)

	cacheDir := config.DownloadDir

	realScanCount := 0
	for _, id := range ids {
		emailBytes, lErr := msg.Read(cacheDir, id)
		if lErr != nil {
			continue
		}

		localEmail, errMail := mail.ReadMessage(bytes.NewReader(emailBytes))
		if errMail == nil {
			fromHeader := localEmail.Header.Get("From")
			sender := email.ParseEmailAddress(fromHeader)
			if cfg_g.IsWhitelisted(sender, config.Whitelist) || cfg_g.IsBlacklisted(sender, config.Blacklist) {
				continue
			}
		}

		cached := false
		if info, err := msg.GetInfo(cacheDir, id); err == nil && info.Classified {
			cached = true
		}

		if !cached {
			realScanCount++
		}
	}

	if !app.TuiActive && realScanCount > 0 {
		fmt.Printf("%s Scanning %d email(s) with Bogofilter in parallel...\n", app.PrefixInfo, realScanCount)
	}

	for _, id := range ids {
		mu.Lock()
		if scanErr != nil {
			mu.Unlock()
			break
		}
		mu.Unlock()

		wg.Add(1)
		go func(msgID string) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			if config.Verbose && !app.TuiActive {
				fmt.Printf("    %s Requesting Bogofilter scan for message %s...\n", app.PrefixInfo, msgID)
			}

			exists, _ := msg.Exists(cacheDir, msgID)
			if !exists {
				mu.Lock()
				if scanErr == nil {
					scanErr = fmt.Errorf("cached email file for %s is missing or empty", msgID)
				}
				mu.Unlock()
				return
			}

			emailBytes, err := msg.Read(cacheDir, msgID)
			if err != nil {
				mu.Lock()
				if scanErr == nil {
					scanErr = fmt.Errorf("failed to read cached email %s: %w", msgID, err)
				}
				mu.Unlock()
				return
			}

			localEmail, errMail := mail.ReadMessage(bytes.NewReader(emailBytes))
			var subject string
			if errMail == nil {
				subject = email.DecodeHeader(dec, localEmail.Header.Get("Subject"))
				fromHeader := localEmail.Header.Get("From")
				sender := email.ParseEmailAddress(fromHeader)
				if cfg_g.IsWhitelisted(sender, config.Whitelist) {
					if config.Verbose && !app.TuiActive {
						fmt.Printf("    %s Message %s sender '%s' is whitelisted. Bypassing all checks.\n", app.PrefixSuccess, msgID, sender)
					}

					TrainHamLocally(msgID, emailBytes, config)

					spamResp := Response{
						Action: "no action (Whitelisted Sender)",
						Score:  0.0,
					}
					mu.Lock()
					scanResults[msgID] = &spamResp
					_ = msg.ClearClassification(cacheDir, msgID)
					mu.Unlock()
					return
				}
				if cfg_g.IsBlacklisted(sender, config.Blacklist) {
					if config.Verbose && !app.TuiActive {
						fmt.Printf("    %s Message %s sender '%s' is blacklisted. Bypassing Bogofilter check and moving to spam learn folder.\n", app.PrefixWarn, msgID, sender)
					}

					spamResp := Response{
						Action: "reject (Blacklisted Sender)",
						Score:  20.0,
					}
					mu.Lock()
					scanResults[msgID] = &spamResp
					blacklistedIDs = append(blacklistedIDs, msgID)
					_ = msg.SetClassification(cacheDir, msgID, true, false, true, 20.0)
					mu.Unlock()
					return
				}
			}

			var spamResp Response
			loadedFromCache := false
			var info *msg.Info

			if inf, iErr := msg.GetInfo(cacheDir, msgID); iErr == nil && inf.Classified {
				slog.Info("Loaded cached scan result", slog.String("message_id", msgID))
				loadedFromCache = true
				info = inf
				spamResp = Response{
					Action: "no action",
					Score:  float64(info.SpamScore),
				}
				if info.IsSpam {
					spamResp.Action = "reject"
				}
			}

			isSpam := false
			isPoliticalSpam := false
			isBlacklisted := false

			if loadedFromCache && info != nil {
				mu.Lock()
				isSpam = info.IsSpam
				isBlacklisted = info.IsBlacklisted
				scanResults[msgID] = &spamResp
				if isSpam {
					spamIDs = append(spamIDs, msgID)
				}
				if isBlacklisted {
					blacklistedIDs = append(blacklistedIDs, msgID)
				}
				mu.Unlock()
			} else {
				cr := classifyWithBogofilter(emailBytes)
				action := "no action"
				if cr.IsSpam {
					action = "reject"
				}
				spamResp = Response{
					Action: action,
					Score:  cr.Score,
					Symbols: map[string]Symbol{
						"BOGOFILTER": {
							Name:        cr.Name,
							Score:       cr.Score,
							Description: cr.Description,
						},
					},
				}

				var bodyStr string
				if localEmail != nil {
					bodyStr, _ = gmail.ExtractPlainBodyText(localEmail)
				}
				bodyStr = email.StripHTML(bodyStr)
				if len(bodyStr) > 8192 {
					bodyStr = bodyStr[:8192]
				}

				isBlockedLangSpam := false
				var blockedChar rune
				var blockedLang string
				subjectRunes := []rune(subject)
				for idx, r := range subjectRunes {
					if !isRuneAllowed(r, config.AllowedLanguages) {
						if unicode.Is(unicode.Greek, r) && !isContiguousGreekRunes(subjectRunes, idx) {
							continue
						}
						isBlockedLangSpam = true
						blockedChar = r
						blockedLang = detectScriptLabel(r)
						break
					}
				}
				if !isBlockedLangSpam {
					fromHeader := localEmail.Header.Get("From")
					fromRunes := []rune(fromHeader)
					for idx, r := range fromRunes {
						if !isRuneAllowed(r, config.AllowedLanguages) {
							if unicode.Is(unicode.Greek, r) && !isContiguousGreekRunes(fromRunes, idx) {
								continue
							}
							isBlockedLangSpam = true
							blockedChar = r
							blockedLang = detectScriptLabel(r)
							break
						}
					}
				}
				if !isBlockedLangSpam {
					bodyRunes := []rune(bodyStr)
					for idx, r := range bodyRunes {
						if !isRuneAllowed(r, config.AllowedLanguages) {
							if unicode.Is(unicode.Greek, r) && !isContiguousGreekRunes(bodyRunes, idx) {
								continue
							}
							isBlockedLangSpam = true
							blockedChar = r
							blockedLang = detectScriptLabel(r)
							break
						}
					}
				}

				isNlpBlocked := false
				var nlpDetectedLang string
				var nlpConfidence float64
				if !isBlockedLangSpam {
					words := strings.Fields(bodyStr)
					if len(words) >= 20 {
						fullText := subject + " " + bodyStr
						cleanedText := cleanTextForNLP(fullText)
						if len(cleanedText) >= 30 {
							info := whatlanggo.Detect(cleanedText)
							if info.Confidence >= 0.50 {
								langName := info.Lang.String()
								langISO := info.Lang.Iso6391()
								if !isDetectedLanguageWhitelisted(langName, langISO, config.AllowedLanguages) {
									isNlpBlocked = true
									nlpDetectedLang = langName
									nlpConfidence = info.Confidence
								}
							}
						}
					}
				}

				var politicalScore float64
				var triggeredIndicators []string
				if !isBlockedLangSpam && !isNlpBlocked && config.BlockPolitical {
					isPoliticalSpam, politicalScore, triggeredIndicators = email.DetectPolitical(subject, bodyStr)
				}

				isAdvertisementSpam := strings.Contains(subject, "פרסומת")

				mu.Lock()
				if isBlockedLangSpam {
					spamResp.Score += 20.0
					spamResp.Action = fmt.Sprintf("reject (Blocked Language Script: %s)", blockedLang)
					if spamResp.Symbols == nil {
						spamResp.Symbols = make(map[string]Symbol)
					}
					symKey := "CUSTOM_LANGUAGE_BLOCK"
					spamResp.Symbols[symKey] = Symbol{
						Name:        symKey,
						Score:       20.0,
						Description: fmt.Sprintf("Contains blocked language character '%c' (Unicode U+%04X in script: %s)", blockedChar, blockedChar, blockedLang),
					}
				} else if isNlpBlocked {
					spamResp.Score += 20.0
					spamResp.Action = fmt.Sprintf("reject (Unallowed Language NLP: %s)", nlpDetectedLang)
					if spamResp.Symbols == nil {
						spamResp.Symbols = make(map[string]Symbol)
					}
					symKey := "CUSTOM_LANGUAGE_NLP_BLOCK"
					spamResp.Symbols[symKey] = Symbol{
						Name:        symKey,
						Score:       20.0,
						Description: fmt.Sprintf("NLP detected language '%s' (Confidence %.1f%%) which is not in AllowedLanguages", nlpDetectedLang, nlpConfidence*100),
					}
				} else if isAdvertisementSpam {
					spamResp.Score += 20.0
					spamResp.Action = "reject (Advertisement Keyword Block)"
					if spamResp.Symbols == nil {
						spamResp.Symbols = make(map[string]Symbol)
					}
					symKey := "CUSTOM_ADVERTISEMENT_BLOCK"
					spamResp.Symbols[symKey] = Symbol{
						Name:        symKey,
						Score:       20.0,
						Description: "Subject contains the blocked advertisement keyword 'פרסומת'",
					}
				} else if isPoliticalSpam {
					spamResp.Score += 20.0
					spamResp.Action = "reject (Political Donation Block)"
					if spamResp.Symbols == nil {
						spamResp.Symbols = make(map[string]Symbol)
					}
					symKey := "CUSTOM_POLITICAL_BLOCK"
					spamResp.Symbols[symKey] = Symbol{
						Name:        symKey,
						Score:       20.0,
						Description: fmt.Sprintf("Weighted score: %.1f/10.0 (triggered: %s)", politicalScore, strings.Join(triggeredIndicators, ", ")),
					}
				}

				scanResults[msgID] = &spamResp

				if config.ScoreThreshold > 0.0 {
					isSpam = spamResp.Score >= config.ScoreThreshold
				} else {
					action := strings.ToLower(spamResp.Action)
					isSpam = strings.HasPrefix(action, "reject") || action == "add header" || action == "rewrite subject"
				}
				if isAdvertisementSpam {
					isSpam = true
				}
				if isSpam {
					spamIDs = append(spamIDs, msgID)
				}

				_ = msg.SetClassification(cacheDir, msgID, isSpam, isPoliticalSpam, strings.Contains(spamResp.Action, "Blacklisted Sender"), float32(spamResp.Score))
				mu.Unlock()
			}

			if config.Verbose && !app.TuiActive {
				fmt.Printf("    %s Completed Bogofilter scan for message %s: Score %.2f (Action: %s)\n", app.PrefixSuccess, msgID, spamResp.Score, spamResp.Action)
				if isSpam {
					fmt.Printf("      -> Marked as SPAM because:\n")
					if loadedFromCache && info != nil {
						fmt.Printf("         [Cache] Loaded from cache.\n")
						if info.IsBlacklisted {
							fmt.Printf("         [Cache] Sender is blacklisted.\n")
						}
						if info.IsPolitical {
							fmt.Printf("         [Cache] Identified as political spam.\n")
						}
						if info.SpamScore > 0 {
							fmt.Printf("         [Cache] Spam score is %.2f.\n", info.SpamScore)
						}
					} else {
						if len(spamResp.Symbols) > 0 {
							for _, sym := range spamResp.Symbols {
								fmt.Printf("         * %s: %s (score: %.2f)\n", sym.Name, sym.Description, sym.Score)
							}
						} else {
							fmt.Printf("         * Custom/Heuristics match (Action: %s)\n", spamResp.Action)
						}
					}
				}
			}
		}(id)
	}

	wg.Wait()

	if scanErr != nil {
		return nil, nil, nil, scanErr
	}

	return spamIDs, blacklistedIDs, scanResults, nil
}
