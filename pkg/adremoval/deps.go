package adremoval

import (
	"abs/pkg/audio"
	"abs/pkg/backend"
	"abs/pkg/config"
	"abs/pkg/detect"
	"abs/pkg/format"
	"abs/pkg/gemini"
	"abs/pkg/pipeline"
	"abs/pkg/types"
	"abs/pkg/util"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const defaultGeminiChunkSec = gemini.DefaultGeminiChunkSec

func getABSClient(cfg Config, quiet bool) (*backend.AudiobookshelfBackend, error) {
	return backend.NewAudiobookshelf(backend.Config{
		Host:        cfg.AudiobookshelfURL,
		User:        cfg.AudiobookshelfUser,
		Pass:        cfg.AudiobookshelfPass,
		Token:       cfg.AudiobookshelfToken,
		DBPath:      cfg.AudiobookshelfDBPath,
		PodcastsDir: cfg.PodcastsDir,
		Quiet:       quiet,
	}), nil
}

func handleTranscribeMin(sourceAudioFile *string, totalDuration float64, cli CLIOptions) float64 {
	dur, _ := pipeline.HandleTranscribeMin(sourceAudioFile, totalDuration, cli.TranscribeMin)
	return dur
}

func isGeminiEngine(cfg Config, cli CLIOptions) bool {
	if cli.WhisperEngine == string(WhisperEngineGemini) {
		return true
	}
	wp := config.GetActiveWhisperProfile(&cfg)
	return wp.Engine == WhisperEngineGemini
}

func removeWorkDirs(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			if entry.Name() == ".work" {
				_ = os.RemoveAll(filepath.Join(dir, entry.Name()))
			} else {
				removeWorkDirs(filepath.Join(dir, entry.Name()))
			}
		}
	}
}

func updateStatusAdDetection(mainMP3File string, successful bool, status, model, errMsg string) {
	_ = pipeline.UpdateEpisodeStatus(mainMP3File, func(st *EpisodeStatusFile) {
		st.AdDetectionSuccessful = &successful
		st.AdDetectionStatus = status
		st.AdDetectionModel = model
		st.AdDetectionError = errMsg
		if !successful {
			st.Status = StateNeedsAdR
		}
	})
}

func updateTranscriptAdDetectionStatus(jsonFile string, successful bool, status, model, errMsg string, adCount int) error {
	if !util.FileExists(jsonFile) {
		return nil
	}
	raw, err := os.ReadFile(jsonFile)
	if err != nil {
		return err
	}
	var data map[string]interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		return err
	}
	data["ad_detection_successful"] = successful
	data["ad_detection_status"] = status
	if model != "" {
		data["ad_detection_model"] = model
	}
	if errMsg != "" {
		data["ad_detection_error"] = errMsg
	} else {
		delete(data, "ad_detection_error")
	}
	if successful {
		data["ad_segments_count"] = adCount
	}
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(jsonFile, append(content, '\n'), 0644)
}

// fatalError reports a condition the engine cannot proceed past and stops.
func fatalError(formatStr string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, formatStr, args...)
	os.Exit(1)
}

// getBackend builds the Audiobookshelf client the engine uses to sync episode
// durations back after a cut.
func getBackend(cfg Config, quiet bool) (backend.Backend, error) {
	return backend.New("audiobookshelf", backend.Config{
		Host:        cfg.AudiobookshelfURL,
		User:        cfg.AudiobookshelfUser,
		Pass:        cfg.AudiobookshelfPass,
		Token:       cfg.AudiobookshelfToken,
		DBPath:      cfg.AudiobookshelfDBPath,
		PodcastsDir: cfg.PodcastsDir,
		Quiet:       quiet,
	})
}

func selectProfile(cfg Config, query string) LLMProfile {
	p, _ := config.SelectLLMProfile(&cfg, query)
	return p
}

func convertJSONToSRT(inputFile string, data *TranscriptionData, customPath string, quiet bool) string {
	res, _ := format.ConvertJSONToSRT(inputFile, data, customPath, quiet)
	return res
}

func convertJSONToTXT(inputFile string, data *TranscriptionData, totalDuration float64, customPath string, quiet bool) string {
	res, _ := format.ConvertJSONToTXT(inputFile, data, totalDuration, customPath, quiet)
	return res
}

func resolveAudioFiles(inputFile string, cli CLIOptions) (string, string, string) {
	return pipeline.ResolveAudioFiles(inputFile, cli.Verbose)
}

func resolveOutputFile(mainMP3File string, cli CLIOptions, totalFiles int) string {
	return pipeline.ResolveOutputFile(mainMP3File, cli.Output, totalFiles)
}

func loadPodcastConfig(dir string) PodcastConfig {
	return config.LoadPodcastConfig(dir, config.PodcastConfig{})
}

func getActiveWhisperProfile(cfg Config) types.WhisperProfile {
	return config.GetActiveWhisperProfile(&cfg)
}

func resolveGeminiAPIKey(cfg Config) string {
	return config.ResolveGeminiAPIKey(&cfg)
}

func detectAdsLLM(transcriptText string, profile LLMProfile) ([]AdSegment, error) {
	return detect.DetectAdsLLM(transcriptText, profile, profile.APIKey)
}

func cutAudioFFmpegWithHost(inputFile string, keepSegments [][2]float64, outputFile, remoteHost string) bool {
	return audio.CutAudioFFmpeg(inputFile, keepSegments, outputFile)
}

func isPodfetchActive(cfg Config) bool { return backend.IsPodfetchActive(&cfg) }

func isAudiobookshelfActive(cfg Config) bool { return backend.IsAudiobookshelfActive(&cfg) }

func normalizeWhisperProfile(p types.WhisperProfile) types.WhisperProfile {
	return config.NormalizeWhisperProfile(p)
}

func whisperEngineBadge(engine types.WhisperEngine) string {
	return config.WhisperEngineBadge(engine)
}
