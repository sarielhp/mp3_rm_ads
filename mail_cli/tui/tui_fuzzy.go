package tui

import (
	"math"
	"sort"
	"strings"
	"unicode"

	"mail_cli/uicommon"
)

type fuzzyMatch struct {
	email uicommon.FolderEmail
	score int
}

func fuzzyScore(query, target string) int {
	query = strings.ToLower(query)
	target = strings.ToLower(target)

	if query == "" {
		return 0
	}

	qi := 0
	score := 0
	prevMatched := false
	wordBoundary := true

	for ti, r := range target {
		if qi >= len(query) {
			break
		}
		qc := rune(query[qi])
		if r == qc {
			score += 10
			if prevMatched {
				score += 5
			}
			if wordBoundary {
				score += 3
			}
			if ti == 0 {
				score += 2
			}
			prevMatched = true
			qi++
		} else {
			prevMatched = false
		}
		wordBoundary = !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}

	if qi < len(query) {
		return 0
	}

	return score
}

func fuzzyMatchEmail(query string, e uicommon.FolderEmail) int {
	score := fuzzyScore(query, e.Subject)
	if score == 0 {
		score = fuzzyScore(query, e.FromRaw)
	}
	if score == 0 {
		score = fuzzyScore(query, e.FromEmail)
	}
	if score == 0 {
		score = fuzzyScore(query, e.ID)
	}
	return score
}

func fuzzyFilterEmails(query string, emails []uicommon.FolderEmail) []uicommon.FolderEmail {
	if query == "" {
		return emails
	}

	var matches []fuzzyMatch
	for _, e := range emails {
		if score := fuzzyMatchEmail(query, e); score > 0 {
			matches = append(matches, fuzzyMatch{email: e, score: score})
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		return matches[i].email.EmailDate.After(matches[j].email.EmailDate)
	})

	result := make([]uicommon.FolderEmail, 0, len(matches))
	for _, m := range matches {
		result = append(result, m.email)
	}
	return result
}

func fuzzyMatchBody(query, body string) []int {
	query = strings.ToLower(query)
	body = strings.ToLower(body)
	if query == "" || body == "" {
		return nil
	}

	type pos struct {
		idx     int
		score   int
		prevIdx int
	}

	dp := make([]pos, len(query))

	for i := range dp {
		dp[i] = pos{idx: -1, score: -math.MaxInt32, prevIdx: -1}
	}

	for bi, r := range body {
		qc := rune(query[0])
		if r == qc {
			s := 10
			if bi == 0 {
				s += 2
			}
			if bi > 0 && (!unicode.IsLetter(rune(body[bi-1])) && !unicode.IsDigit(rune(body[bi-1]))) {
				s += 3
			}
			if s > dp[0].score {
				dp[0] = pos{idx: bi, score: s, prevIdx: -1}
			}
		}
	}

	for qi := 1; qi < len(query); qi++ {
		qc := rune(query[qi])
		best := pos{idx: -1, score: -math.MaxInt32, prevIdx: -1}
		for bi, r := range body {
			if r != qc {
				continue
			}
			s := 10
			if dp[qi-1].idx >= 0 && bi > dp[qi-1].idx {
				base := dp[qi-1].score
				if bi == dp[qi-1].idx+1 {
					base += 5
				}
				if bi > 0 && (!unicode.IsLetter(rune(body[bi-1])) && !unicode.IsDigit(rune(body[bi-1]))) {
					base += 3
				}
				if bi == 0 {
					base += 2
				}
				s = base
			}
			if s > best.score {
				best = pos{idx: bi, score: s, prevIdx: dp[qi-1].idx}
			}
		}
		dp[qi] = best
	}

	if dp[len(query)-1].idx < 0 {
		return nil
	}

	positions := make([]int, len(query))
	cur := len(query) - 1
	for cur >= 0 {
		positions[cur] = dp[cur].idx
		cur--
	}

	return positions
}

func fuzzySplitByMatches(body string, positions []int) []string {
	if len(positions) == 0 {
		return []string{body}
	}

	posSet := make(map[int]bool)
	for _, p := range positions {
		posSet[p] = true
	}

	var parts []string

	idx := 0
	var matchBuf strings.Builder
	var normalBuf strings.Builder
	inMatch := false

	runes := []rune(body)
	for i, r := range runes {
		if posSet[i] {
			if !inMatch {
				if normalBuf.Len() > 0 {
					parts = append(parts, normalBuf.String())
					normalBuf.Reset()
				}
				inMatch = true
			}
			matchBuf.WriteRune(r)
		} else {
			if inMatch {
				if matchBuf.Len() > 0 {
					parts = append(parts, matchBuf.String())
					matchBuf.Reset()
				}
				inMatch = false
			}
			normalBuf.WriteRune(r)
		}
		idx++
	}
	if matchBuf.Len() > 0 {
		parts = append(parts, matchBuf.String())
	} else if normalBuf.Len() > 0 {
		parts = append(parts, normalBuf.String())
	}

	return parts
}
