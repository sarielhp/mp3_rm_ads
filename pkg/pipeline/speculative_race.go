package pipeline

import (
	"context"
	"fmt"
	"time"

	"abs/pkg/config"
	"abs/pkg/gemini"
	"abs/pkg/transcribe"
	"abs/pkg/types"
	"abs/pkg/util"
)

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
	transcribe.AnnounceStart(totalDuration, opts.Quiet)
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
	localWp := transcribe.ResolveWhisperProfileForLanguage(cfg, transcribe.WhisperTargetLanguage(cfg, isHebrew, whisperLang))
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

	return AwaitRaceResults(ctx, cancel, geminiCh, localCh, localWp, opts.Quiet)
}

func AwaitRaceResults(ctx context.Context, cancel context.CancelFunc, geminiCh <-chan GeminiRaceResult, localCh <-chan LocalRaceResult, localWp types.WhisperProfile, quiet bool) (*types.TranscriptionData, []types.AdSegment, bool, error) {
	var geminiRes *GeminiRaceResult
	var localRes *LocalRaceResult

	for geminiRes == nil || localRes == nil {
		select {
		case gr := <-geminiCh:
			geminiRes = &gr
			if gr.Err == nil {
				cancel()
				if !quiet {
					fmt.Println("\n" + util.BoldGreen("Transcription complete: using Gemini result (including ad detection)."))
				}
				return gr.TD, gr.Ads, true, nil
			}
			if !quiet {
				fmt.Printf("\n%s\n", util.BoldYellow(fmt.Sprintf("Transcription: Gemini failed (%v)", gr.Err)))
				target := localWp.URL
				if target == "" {
					target = localWp.Name
				}
				if target == "" {
					target = "local service"
				}
				fmt.Printf("   ➔ %s\n\n", util.Bold(fmt.Sprintf("Continuing with running Whisper service (%s: %s)...", localWp.Name, target)))
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
					fmt.Println("\n" + util.BoldGreen("Transcription complete: using Whisper result."))
				}
				return lr.TD, nil, false, nil
			}
			if !quiet {
				fmt.Printf("\n%s\n", util.BoldYellow(fmt.Sprintf("Transcription: Whisper failed (%v)", lr.Err)))
				fmt.Printf("   ➔ %s\n\n", util.Bold("Continuing with running Gemini service..."))
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
