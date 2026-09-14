package transcribe

import (
	"context"
	"fmt"
	"strings"

	"pod/pkg/audio"
	"pod/pkg/types"
)

// Options holds tuning and metadata options for speech transcription.
type Options struct {
	Prompt          string
	Language        string
	SpeedFactor     float64
	DockerContainer string
	ChunkDuration   int
	ForceChunks     bool
	Quiet           bool
	Verbose         bool
}

// Transcriber defines the interface for converting audio into timestamped transcript data.
type Transcriber interface {
	Transcribe(ctx context.Context, audioPath string, opts Options) (*types.TranscriptionData, error)
}

// WhisperCLITranscriber transcribes audio using a local whisper-cli binary.
type WhisperCLITranscriber struct {
	Profile types.WhisperProfile
}

func NewWhisperCLITranscriber(profile types.WhisperProfile) *WhisperCLITranscriber {
	return &WhisperCLITranscriber{Profile: profile}
}

func (t *WhisperCLITranscriber) Transcribe(ctx context.Context, audioPath string, opts Options) (*types.TranscriptionData, error) {
	td, err := RunWhisperCLITranscriptionContext(ctx, audioPath, t.Profile, opts.Quiet, opts.Verbose, opts.Prompt, opts.Language)
	if err == nil && td != nil {
		StampBackend(td, t.Profile.Engine, t.Profile.Model)
	}
	return td, err
}

// WhisperHTTPTranscriber transcribes audio using a remote Whisper HTTP API server (e.g. Docker GPU).
type WhisperHTTPTranscriber struct {
	Profile types.WhisperProfile
}

func NewWhisperHTTPTranscriber(profile types.WhisperProfile) *WhisperHTTPTranscriber {
	return &WhisperHTTPTranscriber{Profile: profile}
}

func (t *WhisperHTTPTranscriber) Transcribe(ctx context.Context, audioPath string, opts Options) (*types.TranscriptionData, error) {
	totalDuration := audio.GetAudioDuration(audioPath)
	chunkDuration := opts.ChunkDuration
	useChunks := opts.ForceChunks || (chunkDuration > 0 && totalDuration > float64(chunkDuration)*1.5)

	var (
		td  *types.TranscriptionData
		err error
	)

	if useChunks {
		td, err = TranscribeChunksContext(
			ctx, audioPath, t.Profile.URL, opts.Quiet, opts.Verbose,
			totalDuration, opts.SpeedFactor, chunkDuration,
			opts.DockerContainer, opts.Prompt, opts.Language,
		)
	} else {
		td, err = TranscribeWhisperContext(
			ctx, audioPath, t.Profile.URL, opts.Quiet, opts.Verbose,
			totalDuration, opts.SpeedFactor, opts.DockerContainer,
			opts.Prompt, opts.Language, nil,
		)
		if err != nil && strings.Contains(err.Error(), "failed to") && totalDuration > 300 {
			chunkDur := chunkDuration
			if chunkDur <= 0 {
				chunkDur = 900
			}
			td, err = TranscribeChunksContext(
				ctx, audioPath, t.Profile.URL, opts.Quiet, opts.Verbose,
				totalDuration, opts.SpeedFactor, chunkDur,
				opts.DockerContainer, opts.Prompt, opts.Language,
			)
		}
	}

	if err == nil && td != nil {
		StampBackend(td, t.Profile.Engine, t.Profile.Model)
	}
	return td, err
}

// FallbackTranscriber executes Primary and falls back to Secondary if Primary encounters an error.
type FallbackTranscriber struct {
	Primary    Transcriber
	Secondary  Transcriber
	OnFallback func(primaryErr error)
}

func NewFallbackTranscriber(primary, secondary Transcriber, onFallback func(error)) *FallbackTranscriber {
	return &FallbackTranscriber{
		Primary:    primary,
		Secondary:  secondary,
		OnFallback: onFallback,
	}
}

func (f *FallbackTranscriber) Transcribe(ctx context.Context, audioPath string, opts Options) (*types.TranscriptionData, error) {
	td, err := f.Primary.Transcribe(ctx, audioPath, opts)
	if err == nil {
		return td, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if f.OnFallback != nil {
		f.OnFallback(err)
	}
	if f.Secondary != nil {
		return f.Secondary.Transcribe(ctx, audioPath, opts)
	}
	return nil, fmt.Errorf("primary transcriber failed: %w", err)
}
