package detect

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"abs/pkg/config"
	"abs/pkg/gemini"
	"abs/pkg/transcribe"
	"abs/pkg/types"
)

func ContainsHebrew(s string) bool {
	for _, r := range s {
		if (r >= 0x0590 && r <= 0x05FF) || (r >= 0xFB1D && r <= 0xFB4F) {
			return true
		}
	}
	return false
}

func IsHebrewAudio(audioPath string, id3Tags map[string]string, whisperLang string) bool {
	if strings.EqualFold(whisperLang, "he") || strings.EqualFold(whisperLang, "heb") || strings.EqualFold(whisperLang, "hebrew") {
		return true
	}
	if ContainsHebrew(audioPath) {
		return true
	}
	for _, v := range id3Tags {
		if ContainsHebrew(v) {
			return true
		}
	}
	return false
}

func WhisperProfileSupportsLanguage(wp types.WhisperProfile, lang string) bool {
	if lang == "" || lang == "auto" {
		return true
	}
	if len(wp.Languages) == 0 {
		if wp.Engine == types.WhisperEngineLocal && (strings.Contains(wp.Model, ".en") || wp.Model == "tiny.en") {
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

// WhisperTargetLanguage decides which language the Whisper backend has to
// support: an explicitly requested language wins, Hebrew detection comes
// next, and anything else is assumed to be English.
func WhisperTargetLanguage(cfg types.Config, isHebrew bool, requested string) string {
	if requested != "" && !strings.EqualFold(requested, "auto") {
		return requested
	}
	if isHebrew {
		return "he"
	}
	if cfg.WhisperLanguage != "" && !strings.EqualFold(cfg.WhisperLanguage, "auto") {
		return cfg.WhisperLanguage
	}
	return "en"
}

// WhisperProfileUsable reports whether a profile can actually run, so
// speed-based routing never picks a local program whose binary or model file
// is missing. It is a variable so tests can route without touching the host.
var WhisperProfileUsable = func(wp types.WhisperProfile) bool {
	if wp.Engine != types.WhisperEngineLocal {
		return wp.URL != ""
	}
	bin := transcribe.ResolveWhisperCLIBinary(wp.CliBinary)
	if _, err := exec.LookPath(bin); err != nil {
		return false
	}
	_, err := transcribe.ResolveWhisperModelPath(wp.Model)
	return err == nil
}

// ResolveWhisperProfileForLanguage picks the fastest usable Whisper backend
// that supports lang. Speed decides rather than the configured default,
// because the backends differ by an order of magnitude: English routes to the
// local whisper-cli program, while Hebrew, which the English-only models
// cannot handle, falls back to the Docker server.
func ResolveWhisperProfileForLanguage(cfg types.Config, lang string) types.WhisperProfile {
	active := config.GetActiveWhisperProfile(&cfg)
	var best types.WhisperProfile
	found := false
	for _, p := range cfg.WhisperProfiles {
		wp := config.NormalizeWhisperProfile(p)
		if wp.Engine == types.WhisperEngineGemini || !WhisperProfileSupportsLanguage(wp, lang) || !WhisperProfileUsable(wp) {
			continue
		}
		if !found || fasterWhisperProfile(wp, best, active) {
			best, found = wp, true
		}
	}
	if found {
		return best
	}
	if active.Engine == types.WhisperEngineGemini {
		fallbackCfg := config.PrepareWhisperFallbackConfig(cfg)
		return config.GetActiveWhisperProfile(&fallbackCfg)
	}
	return active
}

func fasterWhisperProfile(candidate, best, active types.WhisperProfile) bool {
	if candidate.SpeedFactor != best.SpeedFactor {
		return candidate.SpeedFactor > best.SpeedFactor
	}
	return candidate.ID == active.ID
}

// ResolveLocalWhisperProfile keeps the Hebrew/English shorthand used by the
// speculative race, where Gemini is already the other racer.
func ResolveLocalWhisperProfile(cfg types.Config, isHebrew bool) types.WhisperProfile {
	return ResolveWhisperProfileForLanguage(cfg, WhisperTargetLanguage(cfg, isHebrew, ""))
}

type GeminiRaceResult struct {
	TD  *types.TranscriptionData
	Ads []types.AdSegment
	Err error
}

type LocalRaceResult struct {
	TD  *types.TranscriptionData
	Err error
}

func RunLocalCandidateTranscription(ctx context.Context, audioPath string, wp types.WhisperProfile, cfg types.Config, opts types.ProcOptions, totalDuration, speedFactor float64, whisperPrompt, whisperLang, dockerContainer string) (td *types.TranscriptionData, err error) {
	defer func() { transcribe.StampBackend(td, wp.Engine, wp.Model) }()
	if wp.Engine == types.WhisperEngineLocal {
		return transcribe.RunWhisperCLITranscriptionContext(ctx, audioPath, wp, opts.Quiet, opts.Verbose, whisperPrompt, whisperLang)
	}
	transcribe.AnnounceWhisperServer(wp.URL, wp.Engine, dockerContainer, opts.Quiet)
	chunkDuration := cfg.ChunkDurationSec
	useChunks := opts.UseChunks || (chunkDuration > 0 && totalDuration > float64(chunkDuration)*1.5)
	if useChunks {
		return transcribe.TranscribeChunksContext(
			ctx,
			audioPath, wp.URL, opts.Quiet, opts.Verbose,
			totalDuration, speedFactor, chunkDuration,
			dockerContainer, whisperPrompt, whisperLang,
		)
	}
	return transcribe.TranscribeWhisperContext(
		ctx, audioPath, wp.URL, opts.Quiet, opts.Verbose,
		totalDuration, speedFactor, dockerContainer,
		whisperPrompt, whisperLang, nil,
	)
}

func RunSpeculativeParallelRace(parentCtx context.Context, audioPath string, cfg types.Config, opts types.ProcOptions, totalDuration, speedFactor float64, whisperPrompt, whisperLang, dockerContainer string, isHebrew bool) (*types.TranscriptionData, []types.AdSegment, bool, error) {
	var (
		ctx    context.Context
		cancel context.CancelFunc
	)
	if _, hasDeadline := parentCtx.Deadline(); !hasDeadline {
		ctx, cancel = context.WithTimeout(parentCtx, 30*time.Minute)
	} else {
		ctx, cancel = context.WithCancel(parentCtx)
	}
	defer cancel()

	if isHebrew && whisperLang == "" {
		whisperLang = "he"
	}
	localWp := ResolveWhisperProfileForLanguage(cfg, WhisperTargetLanguage(cfg, isHebrew, whisperLang))
	if isHebrew && !opts.Quiet {
		fmt.Printf("   Hebrew detected: routed local Whisper to %s (%s)\n", localWp.Name, config.WhisperEngineBadge(localWp.Engine))
	}

	geminiCh := make(chan GeminiRaceResult, 1)
	localCh := make(chan LocalRaceResult, 1)

	chunkDur := gemini.DefaultGeminiChunkSec
	if cfg.ChunkDurationSec > 0 {
		chunkDur = float64(cfg.ChunkDurationSec)
	}

	go func() {
		td, ads, err := gemini.ProcessWithGeminiConfig(ctx, audioPath, cfg, chunkDur)
		transcribe.StampBackend(td, types.WhisperEngineGemini, cfg.GetGeminiModel())
		geminiCh <- GeminiRaceResult{TD: td, Ads: ads, Err: err}
	}()

	go func() {
		td, err := RunLocalCandidateTranscription(ctx, audioPath, localWp, cfg, opts, totalDuration, speedFactor, whisperPrompt, whisperLang, dockerContainer)
		localCh <- LocalRaceResult{TD: td, Err: err}
	}()

	return AwaitRaceResults(ctx, cancel, geminiCh, localCh, opts.Quiet)
}

func AwaitRaceResults(ctx context.Context, cancel context.CancelFunc, geminiCh <-chan GeminiRaceResult, localCh <-chan LocalRaceResult, quiet bool) (*types.TranscriptionData, []types.AdSegment, bool, error) {
	var geminiRes *GeminiRaceResult
	var localRes *LocalRaceResult

	for geminiRes == nil || localRes == nil {
		select {
		case gr := <-geminiCh:
			geminiRes = &gr
			if gr.Err == nil {
				cancel()
				if !quiet {
					fmt.Println("   [Race] Gemini Flash completed first with cuts!")
				}
				return gr.TD, gr.Ads, true, nil
			}
			if !quiet {
				fmt.Printf("   [Race] Gemini returned (%v); awaiting running local Whisper...\n", gr.Err)
			}
			if localRes != nil {
				if localRes.Err == nil {
					return localRes.TD, nil, false, nil
				}
				return nil, nil, false, fmt.Errorf("both Gemini and local Whisper failed: gemini=%v, local=%v", gr.Err, localRes.Err)
			}
		case lr := <-localCh:
			localRes = &lr
			if lr.Err == nil {
				cancel()
				if !quiet {
					fmt.Println("   [Race] Local Whisper completed first!")
				}
				return lr.TD, nil, false, nil
			}
			if !quiet {
				fmt.Printf("   [Race] Local Whisper failed (%v); awaiting Gemini...\n", lr.Err)
			}
			if geminiRes != nil {
				if geminiRes.Err == nil {
					return geminiRes.TD, geminiRes.Ads, true, nil
				}
				return nil, nil, false, fmt.Errorf("both local Whisper and Gemini failed: local=%v, gemini=%v", lr.Err, geminiRes.Err)
			}
		case <-ctx.Done():
			return nil, nil, false, ctx.Err()
		}
	}
	return nil, nil, false, fmt.Errorf("speculative transcription race finished with no winner")
}
