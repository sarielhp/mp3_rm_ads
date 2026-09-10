package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"abs/pkg/audio"
	"abs/pkg/backend"
	"abs/pkg/config"
	"abs/pkg/detect"
	"abs/pkg/format"
	"abs/pkg/gemini"
	"abs/pkg/pipeline"
	"abs/pkg/player"
	"abs/pkg/podcast"
	"abs/pkg/remote"
	"abs/pkg/transcribe"
	"abs/pkg/types"
	"abs/pkg/util"

	"github.com/fatih/color"
)

type (
	Config              = types.Config
	CLIOptions          = types.CLIOptions
	WhisperConfig       = types.WhisperConfig
	BackendConfig       = types.BackendConfig
	RemoteConfig        = types.RemoteConfig
	PolicyConfig        = types.PolicyConfig
	GeminiConfig        = types.GeminiConfig
	ProcOptions         = types.ProcOptions
	RemoteOptions       = types.RemoteOptions
	PolicyOptions       = types.PolicyOptions
	BackendOptions      = types.BackendOptions
	LLMProfile          = types.LLMProfile
	AdSegment           = types.AdSegment
	TranscriptionData   = types.TranscriptionData
	CutsData            = types.CutsData
	CutEntry            = types.CutEntry
	EpisodeStatusFile   = types.EpisodeStatusFile
	EpisodeAudioMeta    = types.EpisodeAudioMeta
	EpisodeAdCut        = types.EpisodeAdCut
	PlayerTrack         = types.PlayerTrack
	PlayerStatus        = types.PlayerStatusDTO
	Podcast             = backend.Podcast
	PodcastConfig       = config.PodcastConfig
	FeedEpisode         = backend.FeedEpisode
	Episode             = backend.Episode
	WhisperEngine       = types.WhisperEngine
	WhisperProfile      = types.WhisperProfile
	RemoteTransport     = remote.RemoteTransport
	RemoteBatchJobItem  = types.RemoteBatchJobItem
	RemoteDoneManifest  = remote.RemoteDoneManifest
	DefaultSSHTransport = remote.DefaultSSHTransport
	RemoteDoneItem      = remote.RemoteDoneItem
	ResolvedPodcast     = podcast.ResolvedPodcast
	ResolvedEpisode     = podcast.ResolvedEpisode
	ResolvedID          = podcast.ResolvedID
	syncWG              = util.SyncWG
)

type tuiPodcast struct {
	name string
	dir  string
}

type podcastDirEntry struct {
	dir        string
	folderName string
	title      string
	shortID    string
}

const (
	AdRemovalNone   = config.AdRemovalNone
	AdRemovalLatest = config.AdRemovalLatest
	AdRemovalAll    = config.AdRemovalAll

	DownloadPolicyNone    = config.DownloadPolicyNone
	DownloadPolicyLatest  = config.DownloadPolicyLatest
	DownloadPolicyLatestK = config.DownloadPolicyLatestK
	DownloadPolicyAll     = config.DownloadPolicyAll

	StateAwaitingTranscription = types.StateAwaitingTranscription
	StateTranscribingLocally   = types.StateTranscribingLocally
	StateTranscribingRemotely  = types.StateTranscribingRemotely
	StateNeedsAdR              = types.StateNeedsAdR
	StateCuttingLocally        = types.StateCuttingLocally
	StateCuttingRemotely       = types.StateCuttingRemotely
	StateQueuedRemote          = types.StateQueuedRemote
	StateReadyForCopyBack      = types.StateReadyForCopyBack
	StateCopiedBack            = types.StateCopiedBack
	StateDone                  = types.StateDone
	StateArchived              = types.StateArchived
	StateFailed                = types.StateFailed
	StateDownloaded            = types.StateDownloaded

	BatchStatusQueued     = types.BatchStatusQueued
	BatchStatusProcessing = types.BatchStatusProcessing
	BatchStatusCompleted  = types.BatchStatusCompleted
	BatchStatusFailed     = types.BatchStatusFailed

	WhisperEngineLocal  = types.WhisperEngineLocal
	WhisperEngineGemini = types.WhisperEngineGemini
	WhisperEngineDocker = types.WhisperEngineDocker

	PlayerSocketPath = player.PlayerSocketPath
)

var (
	globalPlayer = player.GetGlobalPlayer()
	greenCheck   = "\u2713"
)

type mockDimStyle struct{}

func (m mockDimStyle) Render(s string) string { return color.New(color.Faint).Sprint(s) }

var tuiDimStyle mockDimStyle

func fatalError(formatStr string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, formatStr, args...)
	os.Exit(1)
}

func bold(s string) string              { return util.Bold(s) }
func boldYellow(s string) string        { return util.BoldYellow(s) }
func boldGreen(s string) string         { return util.BoldGreen(s) }
func boldCyan(s string) string          { return util.BoldCyan(s) }
func truncate(s string, max int) string { return util.Truncate(s, max) }
func truncateDisplayName(s string, max int) string {
	return util.TruncateDisplayName(s, max)
}
func displayName(s string) string          { return util.DisplayName(s) }
func stripExt(path string) string          { return util.StripExt(path) }
func fileExists(path string) bool          { return util.FileExists(path) }
func safeMove(src, dst string) error       { return util.SafeMove(src, dst) }
func verifyTempFile(path string)           { util.VerifyTempFile(path) }
func copyFile(src, dst string) error       { return util.CopyFileErr(src, dst) }
func findMP3Files(dir string) []string     { return util.FindMP3Files(dir) }
func stripHTML(s string) string            { return backend.StripHTML(s) }
func formatDurationShort(d float64) string { return format.FormatClock(d) }

func wrapText(text string, maxW int) []string {
	if maxW <= 0 {
		maxW = 40
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	curLine := words[0]
	for _, w := range words[1:] {
		if len([]rune(curLine))+1+len([]rune(w)) <= maxW {
			curLine += " " + w
		} else {
			lines = append(lines, curLine)
			curLine = w
		}
	}
	if len(curLine) > 0 {
		lines = append(lines, curLine)
	}
	return lines
}

func formatClock(sec float64) string             { return format.FormatClock(sec) }
func formatSRTTime(sec float64) string           { return format.FormatSRTTime(sec) }
func mergeIntervals(ads []AdSegment) []AdSegment { return format.MergeIntervals(ads) }

func saveCutsJSON(mainFile string, totalDuration float64, adSegments []AdSegment, profile *LLMProfile, quiet bool) types.CutsResult {
	return format.SaveCutsJSON(mainFile, totalDuration, adSegments, profile, quiet)
}

func convertJSONToSRT(inputFile string, data *TranscriptionData, customPath string, quiet bool) string {
	res, _ := format.ConvertJSONToSRT(inputFile, data, customPath, quiet)
	return res
}

func convertJSONToTXT(inputFile string, data *TranscriptionData, totalDuration float64, customPath string, quiet bool) string {
	res, _ := format.ConvertJSONToTXT(inputFile, data, totalDuration, customPath, quiet)
	return res
}

func getAudioDuration(path string) float64   { return audio.GetAudioDuration(path) }
func getMP3DiskDuration(path string) float64 { return audio.GetAudioDuration(path) }

func cutAudioFFmpeg(inputFile string, keepSegments [][2]float64, outputFile string) bool {
	return audio.CutAudioFFmpeg(inputFile, keepSegments, outputFile)
}

func buildWavHeader(dataLen int) []byte { return transcribe.BuildWavHeader(dataLen) }

func loadConfig() Config {
	cfg, err := config.LoadConfig()
	if err != nil || cfg == nil {
		c := config.DefaultConfig()
		return c
	}
	return *cfg
}

func saveConfig(cfg Config) error { return config.SaveConfig(&cfg) }
func ensureConfigExists()         { _, _ = config.EnsureConfigExists() }

func selectProfile(cfg Config, query string) LLMProfile {
	p, _ := config.SelectLLMProfile(&cfg, query)
	return p
}

func loadPodcastConfig(dir string) PodcastConfig {
	return config.LoadPodcastConfig(dir, config.PodcastConfig{})
}

func savePodcastConfig(dir string, cfg PodcastConfig) error {
	return config.SavePodcastConfig(dir, cfg)
}

func normalizeAdRemovalMode(mode string) string    { return config.NormalizeAdRemovalMode(mode) }
func adRemovalModeLabel(mode string) string        { return config.AdRemovalModeLabel(mode) }
func adRemovalModeBadge(mode string) string        { return config.AdRemovalModeBadge(mode) }
func normalizeDownloadPolicy(policy string) string { return config.NormalizeDownloadPolicy(policy) }
func downloadPolicyBadge(policy string, k int) string {
	return config.DownloadPolicyBadge(policy, k)
}
func resolveGeminiAPIKey(cfg Config) string {
	return config.ResolveGeminiAPIKey(&cfg)
}

func loadPodcastCache(dir string) (*podcast.CachedPodcastIndex, error) {
	return podcast.LoadPodcastCache(dir)
}

func loadEpisodeDetails(podDir, epFilename string) (*podcast.CachedEpisodeDetails, error) {
	return podcast.LoadEpisodeDetails(podDir, epFilename)
}

func getEpisodePublicationTime(path string) time.Time {
	return podcast.GetEpisodePublicationTime(path)
}

func getOrCreateEpisodeStatus(path string) *EpisodeStatusFile {
	return pipeline.GetOrCreateEpisodeStatus(path)
}

func isEpisodeCompleted(audioPath string) bool {
	return pipeline.IsEpisodeCompleted(audioPath)
}

func generatePodcastShortID(name string) string { return podcast.GeneratePodcastShortID(name) }

func getOrSetPodcastShortID(podcastDir, title string) string {
	return podcast.GetOrSetPodcastShortID(podcastDir, title)
}

func getOrSetEpisodeShortID(podDir, podShortID, audioPath string) string {
	return podcast.GetOrSetEpisodeShortID(podDir, podShortID, audioPath)
}

func episodeTitleFromPath(audioPath string) string {
	return podcast.EpisodeTitleFromPath(audioPath)
}

func resolveAnyID(podcastsDir, query string) (*ResolvedID, error) {
	return podcast.ResolveAnyID(podcastsDir, query)
}

func scanPodcastDirs(podcastsDir string) []podcastDirEntry {
	res := podcast.ScanPodcastDirs(podcastsDir)
	out := make([]podcastDirEntry, len(res))
	for i, r := range res {
		out[i] = podcastDirEntry{
			dir:        r.Dir,
			folderName: r.FolderName,
			title:      r.Title,
			shortID:    r.ShortID,
		}
	}
	return out
}

func statusPathFor(audioFile string) string { return pipeline.StatusPathFor(audioFile) }

func formatTranscript(data *TranscriptionData, totalDuration float64) string {
	return pipeline.FormatTranscript(data, totalDuration)
}

func loadOrTranscribe(sourceAudioFile, jsonFile string, cfg Config, opts ProcOptions, selectedProfile LLMProfile, totalDuration, speedFactor float64, whisperLanguage, whisperPrompt string, id3TagsOut map[string]string, isNewlyTranscribed *bool, t0Step1 *time.Time) (*TranscriptionData, error) {
	return pipeline.LoadOrTranscribe(sourceAudioFile, jsonFile, cfg, opts, selectedProfile, totalDuration, speedFactor, whisperLanguage, whisperPrompt, id3TagsOut, isNewlyTranscribed, t0Step1)
}

func saveJSONTranscript(mainFile string, data *TranscriptionData, jsonFile string, quiet bool, id3Tags map[string]string) error {
	return pipeline.SaveJSONTranscript(mainFile, data, jsonFile, quiet, id3Tags)
}

func loadTUIPodcasts(podcastsDir string) ([]tuiPodcast, error) {
	entries, err := os.ReadDir(podcastsDir)
	if err != nil {
		return nil, err
	}
	var podcasts []tuiPodcast
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == ".work" || strings.HasPrefix(entry.Name(), ".") || strings.HasSuffix(entry.Name(), "-1") {
			continue
		}
		podPath := filepath.Join(podcastsDir, entry.Name())
		podcasts = append(podcasts, tuiPodcast{name: entry.Name(), dir: podPath})
	}
	return podcasts, nil
}

func saveQueue(dir string, entries []string) error {
	return updateQueue(dir, func([]string) []string {
		return entries
	})
}

func runRemotePush(cfg *Config, args []string, host string, transport remote.RemoteTransport, priority int, quiet, verbose bool) error {
	return remote.RunRemotePush(cfg, args, host, transport, priority, quiet, verbose)
}

func runRemotePull(cfg *Config, remoteHost string, transport remote.RemoteTransport, quiet, verbose bool) error {
	return remote.RunRemotePull(cfg, remoteHost, transport, quiet, verbose)
}

func runRemoteDeploy(cfg *Config, remoteHost string, transport remote.RemoteTransport, quiet, verbose bool) error {
	return remote.RunRemoteDeploy(cfg, remoteHost, transport, quiet, verbose)
}

func runRemoteScan(cfg *Config, targetDir string, ifDirty bool, quiet, verbose bool) error {
	return remote.RunRemoteScan(cfg, targetDir, ifDirty, quiet, verbose)
}

func runRemoteStatus(cfg *Config, remoteHost string, transport remote.RemoteTransport, quiet, verbose bool) error {
	return remote.RunRemoteStatus(cfg, remoteHost, transport, quiet, verbose)
}

func runRemoteStop(cfg *Config, remoteHost string, transport remote.RemoteTransport, quiet, verbose bool) error {
	return remote.RunRemoteStop(cfg, remoteHost, transport, quiet, verbose)
}

func runRemoteCancel(cfg *Config, remoteHost, batchID string, transport remote.RemoteTransport, quiet bool) error {
	return remote.RunRemoteCancel(cfg, remoteHost, batchID, transport, quiet)
}

func runRemoteClear(cfg *Config, remoteHost string, transport remote.RemoteTransport, quiet bool) error {
	return remote.RunRemoteClear(cfg, remoteHost, transport, quiet)
}

func runRemoteWorkerLoop(cfg *Config, targetDir string, daemon bool, quiet, verbose bool) error {
	return remote.RunRemoteWorkerLoop(cfg, targetDir, daemon, quiet, verbose)
}

func runRemoteAck(remoteDir string, relPaths []string) error {
	return remote.RunRemoteAck(remoteDir, relPaths)
}

func ensureRemoteEnvironmentAndWorker(cfg *Config, targetHost, remoteWorkDir string, transport remote.RemoteTransport, quiet bool) error {
	return remote.EnsureRemoteEnvironmentAndWorker(cfg, targetHost, remoteWorkDir, transport, quiet)
}

func ResolveProcessingHost(cfg *Config, hostCandidate string, transport remote.RemoteTransport) (string, bool, error) {
	return remote.ResolveProcessingHost(cfg, hostCandidate, transport)
}

func formatPlayerTime(sec float64) string { return player.FormatPlayerTime(sec) }

func runPlayerDaemon(audioPath, title, podcast string) error {
	return player.RunPlayerDaemon(audioPath, title, podcast)
}

func isPlayerSocketAlive() bool { return player.IsPlayerSocketAlive() }

func StartPlayerTrack(audioPath, title, podcast string) error {
	return player.StartPlayerTrack(audioPath, title, podcast)
}

func PausePlayerSocket() (bool, error)          { return player.PausePlayerSocket() }
func ResumePlayerSocket() error                 { return player.ResumePlayerSocket() }
func StopPlayerSocket() error                   { return player.StopPlayerSocket() }
func QueryPlayerStatus() (*PlayerStatus, error) { return player.QueryPlayerStatus() }

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

func isAudiobookshelfActive(cfg Config) bool { return backend.IsAudiobookshelfActive(&cfg) }

func detectAdsLLM(transcriptText string, profile LLMProfile) ([]AdSegment, error) {
	return detect.DetectAdsLLM(transcriptText, profile, profile.APIKey)
}

func loadManifest(path string) (*types.RemoteBatchManifest, error) {
	return remote.LoadManifest(path)
}

func saveManifest(path string, m *types.RemoteBatchManifest) error {
	return remote.SaveManifest(path, m)
}

func recalculateManifestStats(m *types.RemoteBatchManifest) {
	remote.RecalculateManifestStats(m)
}

func ProcessWithGeminiConfig(ctx context.Context, audioPath string, cfg Config, chunkDurSec float64) (*types.TranscriptionData, []types.AdSegment, error) {
	return gemini.ProcessWithGeminiConfig(ctx, audioPath, cfg, chunkDurSec)
}

func cacheStats() (string, int, int64) {
	return podcast.CacheStats()
}

func resetCache() error {
	return podcast.ResetCache()
}

func testAudiobookshelfServer(cfg Config, quiet bool) bool {
	b, err := getBackend(cfg, quiet)
	if err != nil {
		return false
	}
	ok, _ := b.TestConnection(quiet)
	return ok
}

func saveEpisodeStatus(path string, st *EpisodeStatusFile) error {
	return pipeline.SaveEpisodeStatus(path, st)
}

func loadEpisodeStatus(path string) (*EpisodeStatusFile, error) {
	return pipeline.LoadEpisodeStatus(path)
}

func updateQueue(dir string, mutate func([]string) []string) error {
	return pipeline.UpdateQueue(dir, mutate)
}

func addEpisodeToQueueFile(podDir, filename string) bool {
	return pipeline.AddToQueue(podDir, filename)
}

func removeEpisodeFromQueueFile(podDir, filename string) bool {
	return pipeline.RemoveFromQueue(podDir, filename)
}

func isEpisodeClean(mp3Path string) bool { return pipeline.IsEpisodeClean(mp3Path) }

func getEpisodeDurations(mp3Path string, st *EpisodeStatusFile) (float64, float64) {
	return pipeline.EpisodeDurations(mp3Path, st)
}

func filterMP3FilesByPodcastConfig(files []string, dir string, cfg PodcastConfig) []string {
	return podcast.FilterByAdRemovalPolicy(files, dir, cfg)
}
