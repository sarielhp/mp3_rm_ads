package cli

import (
	"context"
	"fmt"
	"strings"
)

func containsHebrew(s string) bool {
	for _, r := range s {
		if (r >= 0x0590 && r <= 0x05FF) || (r >= 0xFB1D && r <= 0xFB4F) {
			return true
		}
	}
	return false
}

func isHebrewAudio(audioPath string, id3Tags map[string]string, whisperLang string) bool {
	if strings.EqualFold(whisperLang, "he") || strings.EqualFold(whisperLang, "heb") || strings.EqualFold(whisperLang, "hebrew") {
		return true
	}
	if containsHebrew(audioPath) {
		return true
	}
	for _, v := range id3Tags {
		if containsHebrew(v) {
			return true
		}
	}
	return false
}

func whisperProfileSupportsLanguage(wp WhisperProfile, lang string) bool {
	if lang == "" || lang == "auto" {
		return true
	}
	if len(wp.Languages) == 0 {
		if wp.Engine == WhisperEngineLocal && (strings.Contains(wp.Model, ".en") || wp.Model == "tiny.en") {
			return strings.EqualFold(lang, "en")
		}
		return true
	}
	for _, l := range wp.Languages {
		if l == "*" || strings.EqualFold(l, lang) {
			return true
		}
	}
	return false
}

func resolveLocalWhisperProfile(config Config, isHebrew bool) WhisperProfile {
	wp := getActiveWhisperProfile(config)
	if !isHebrew {
		if wp.Engine == WhisperEngineGemini {
			fallbackCfg := prepareWhisperFallbackConfig(config)
			return getActiveWhisperProfile(fallbackCfg)
		}
		return wp
	}
	if whisperProfileSupportsLanguage(wp, "he") && wp.Engine != WhisperEngineGemini {
		return wp
	}
	for _, p := range config.WhisperProfiles {
		if p.Engine == WhisperEngineDocker && whisperProfileSupportsLanguage(p, "he") {
			return normalizeWhisperProfile(p)
		}
	}
	for _, p := range config.WhisperProfiles {
		if p.Engine != WhisperEngineGemini && whisperProfileSupportsLanguage(p, "he") {
			return normalizeWhisperProfile(p)
		}
	}
	return wp
}

type geminiRaceResult struct {
	td  *TranscriptionData
	ads []AdSegment
	err error
}

type localRaceResult struct {
	td  *TranscriptionData
	err error
}

func runLocalCandidateTranscription(ctx context.Context, audioPath string, wp WhisperProfile, config Config, cli CLIOptions, totalDuration, speedFactor float64, whisperPrompt, whisperLang, dockerContainer string) (*TranscriptionData, error) {
	if wp.Engine == WhisperEngineLocal {
		return runWhisperCLITranscriptionContext(ctx, audioPath, wp, cli.Quiet, cli.Verbose, whisperPrompt, whisperLang)
	}
	chunkDuration := config.ChunkDurationSec
	useChunks := cli.UseChunks || (chunkDuration > 0 && totalDuration > float64(chunkDuration)*1.5)
	if useChunks {
		return transcribeChunks(
			audioPath, wp.URL, cli.Quiet, cli.Verbose,
			totalDuration, speedFactor, chunkDuration,
			dockerContainer, whisperPrompt, whisperLang,
		)
	}
	return transcribeWhisperContext(
		ctx, audioPath, wp.URL, cli.Quiet, cli.Verbose,
		totalDuration, speedFactor, dockerContainer,
		whisperPrompt, whisperLang, nil,
	)
}

func runSpeculativeParallelRace(parentCtx context.Context, audioPath string, config Config, cli CLIOptions, totalDuration, speedFactor float64, whisperPrompt, whisperLang, dockerContainer string, isHebrew bool) (*TranscriptionData, []AdSegment, bool, error) {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	if isHebrew && whisperLang == "" {
		whisperLang = "he"
	}
	localWp := resolveLocalWhisperProfile(config, isHebrew)
	if isHebrew && !cli.Quiet {
		fmt.Printf("   Hebrew detected: routed local Whisper to %s (%s)\n", localWp.Name, whisperEngineBadge(localWp.Engine))
	}

	geminiCh := make(chan geminiRaceResult, 1)
	localCh := make(chan localRaceResult, 1)

	chunkDur := defaultGeminiChunkSec
	if config.ChunkDurationSec > 0 {
		chunkDur = float64(config.ChunkDurationSec)
	}

	go func() {
		td, ads, err := ProcessWithGeminiConfig(ctx, audioPath, config, chunkDur)
		geminiCh <- geminiRaceResult{td: td, ads: ads, err: err}
	}()

	go func() {
		td, err := runLocalCandidateTranscription(ctx, audioPath, localWp, config, cli, totalDuration, speedFactor, whisperPrompt, whisperLang, dockerContainer)
		localCh <- localRaceResult{td: td, err: err}
	}()

	return awaitRaceResults(ctx, cancel, geminiCh, localCh, cli.Quiet)
}

func awaitRaceResults(ctx context.Context, cancel context.CancelFunc, geminiCh <-chan geminiRaceResult, localCh <-chan localRaceResult, quiet bool) (*TranscriptionData, []AdSegment, bool, error) {
	var geminiRes *geminiRaceResult
	var localRes *localRaceResult

	for geminiRes == nil || localRes == nil {
		select {
		case gr := <-geminiCh:
			geminiRes = &gr
			if gr.err == nil {
				cancel()
				if !quiet {
					fmt.Println("   [Race] Gemini Flash completed first with cuts!")
				}
				return gr.td, gr.ads, true, nil
			}
			if !quiet {
				fmt.Printf("   [Race] Gemini returned (%v); awaiting running local Whisper...\n", gr.err)
			}
			if localRes != nil {
				if localRes.err == nil {
					return localRes.td, nil, false, nil
				}
				return nil, nil, false, fmt.Errorf("both Gemini and local Whisper failed: gemini=%v, local=%v", gr.err, localRes.err)
			}
		case lr := <-localCh:
			localRes = &lr
			if lr.err == nil {
				cancel()
				if !quiet {
					fmt.Println("   [Race] Local Whisper completed first!")
				}
				return lr.td, nil, false, nil
			}
			if !quiet {
				fmt.Printf("   [Race] Local Whisper failed (%v); awaiting Gemini...\n", lr.err)
			}
			if geminiRes != nil {
				if geminiRes.err == nil {
					return geminiRes.td, geminiRes.ads, true, nil
				}
				return nil, nil, false, fmt.Errorf("both local Whisper and Gemini failed: local=%v, gemini=%v", lr.err, geminiRes.err)
			}
		case <-ctx.Done():
			return nil, nil, false, ctx.Err()
		}
	}
	return nil, nil, false, fmt.Errorf("speculative transcription race finished with no winner")
}
