package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	
	"github.com/fatih/color"
	"github.com/sariel/abs/pkg/audio"
	"github.com/sariel/abs/pkg/backend"
	"github.com/sariel/abs/pkg/config"
	"github.com/sariel/abs/pkg/detect"
	"github.com/sariel/abs/pkg/format"
	"github.com/sariel/abs/pkg/gemini"
	"github.com/sariel/abs/pkg/pipeline"
	"github.com/sariel/abs/pkg/player"
	"github.com/sariel/abs/pkg/podcast"
	"github.com/sariel/abs/pkg/remote"
	"github.com/sariel/abs/pkg/transcribe"
		"github.com/sariel/abs/pkg/types"
	"github.com/sariel/abs/pkg/util"
)

type (
	Config             = types.Config
	CLIOptions         = types.CLIOptions
	LLMProfile         = types.LLMProfile
	AdSegment          = types.AdSegment
	TranscriptionData  = types.TranscriptionData
	CutsData           = types.CutsData
	CutEntry           = types.CutEntry
	EpisodeStatusFile  = types.EpisodeStatusFile
	EpisodeAudioMeta   = types.EpisodeAudioMeta
	EpisodeAdCut       = types.EpisodeAdCut
	PlayerTrack        = types.PlayerTrack
	PlayerStatus       = types.PlayerStatusDTO
	Podcast            = backend.Podcast
	PodcastConfig      = config.PodcastConfig
	FeedEpisode        = backend.FeedEpisode
	Episode            = backend.Episode
	WhisperEngine      = types.WhisperEngine
	WhisperProfile     = types.WhisperProfile
	RemoteTransport    = remote.RemoteTransport
	RemoteBatchJobItem = types.RemoteBatchJobItem
	RemoteDoneManifest = remote.RemoteDoneManifest
	DefaultSSHTransport = remote.DefaultSSHTransport
	RemoteDoneItem     = remote.RemoteDoneItem
		ResolvedPodcast    = podcast.ResolvedPodcast
	ResolvedEpisode    = podcast.ResolvedEpisode
	ResolvedID         = podcast.ResolvedID
	fileLockWrapper    = util.FileLockWrapper
	syncWG             = util.SyncWG
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

const defaultGeminiChunkSec = gemini.DefaultGeminiChunkSec

var (
	globalPlayer  = player.GetGlobalPlayer()
	queueUpdateMu util.SyncMutex
	greenCheck    = "\u2713"
)

type mockDimStyle struct{}

func (m mockDimStyle) Render(s string) string { return color.New(color.Faint).Sprint(s) }

var tuiDimStyle mockDimStyle

func fatalError(formatStr string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, formatStr, args...)
	os.Exit(1)
}

func bold(s string) string               { return util.Bold(s) }
func boldYellow(s string) string         { return util.BoldYellow(s) }
func boldGreen(s string) string          { return util.BoldGreen(s) }
func boldCyan(s string) string           { return util.BoldCyan(s) }
func repeatStr(s string, n int) string   { return util.RepeatStr(s, n) }
func truncate(s string, max int) string  { return util.Truncate(s, max) }
func truncateDisplayName(s string, max int) string {
	return util.TruncateDisplayName(s, max)
}
func displayName(s string) string        { return util.DisplayName(s) }
func stripExt(path string) string        { return util.StripExt(path) }
func fileExists(path string) bool        { return util.FileExists(path) }
func safeMove(src, dst string) error     { return util.SafeMove(src, dst) }
func workDirFor(path string) string      { return util.WorkDirFor(path) }
func verifyTempFile(path string)         { util.VerifyTempFile(path) }
func execCommand(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}
func copyFileErr(src, dst string) error  { return util.CopyFileErr(src, dst) }
func copyFile(src, dst string) error     { return util.CopyFileErr(src, dst) }
func acquireFileLock(path string) (*fileLockWrapper, error) {
	return util.AcquireFileLock(path)
}
func findMP3Files(dir string) []string   { return util.FindMP3Files(dir) }
func stripHTML(s string) string          { return backend.StripHTML(s) }
func formatDurationShort(d float64) string { return format.FormatClock(d) }
func printSeparator()                    { fmt.Println(strings.Repeat("─", 50)) }

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

func formatClock(sec float64) string     { return format.FormatClock(sec) }
func formatTime(sec float64) string      { return format.FormatTime(sec) }
func formatSRTTime(sec float64) string   { return format.FormatSRTTime(sec) }
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

func cutAudioFFmpegWithHost(inputFile string, keepSegments [][2]float64, outputFile, remoteHost string) bool {
	return audio.CutAudioFFmpeg(inputFile, keepSegments, outputFile)
}

func extractID3Tags(path string) map[string]string { return audio.ExtractID3Tags(path) }
func buildWavHeader(dataLen int) []byte             { return transcribe.BuildWavHeader(dataLen) }

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

func normalizeAdRemovalMode(mode string) string      { return config.NormalizeAdRemovalMode(mode) }
func adRemovalModeLabel(mode string) string          { return config.AdRemovalModeLabel(mode) }
func adRemovalModeBadge(mode string) string          { return config.AdRemovalModeBadge(mode) }
func normalizeDownloadPolicy(policy string) string   { return config.NormalizeDownloadPolicy(policy) }
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

func isEpisodeInRemoteFlight(audioPath string) bool {
	return pipeline.IsEpisodeInRemoteFlight(audioPath)
}

func resolvePodcastDirByIDOrName(baseDir, target string) (string, string, bool) {
	return podcast.ResolvePodcastDirByIDOrName(baseDir, target)
}

func resolveAudioFiles(inputFile string, cli CLIOptions) (string, string, string) {
	return pipeline.ResolveAudioFiles(inputFile, cli.Verbose)
}

func resolveOutputFile(mainMP3File string, cli CLIOptions, totalFiles int) string {
	return pipeline.ResolveOutputFile(mainMP3File, cli.Output, totalFiles)
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

func fetchFeedDirect(feedURL, absBaseURL, itemID string) ([]backend.FeedEpisode, string, string, bool, error) {
	return podcast.FetchFeedDirect(feedURL, absBaseURL, itemID)
}

func statusPathFor(audioFile string) string { return pipeline.StatusPathFor(audioFile) }


func updateEpisodeStatus(path string, mutate func(*EpisodeStatusFile)) error {
	return pipeline.UpdateEpisodeStatus(path, mutate)
}

func updateStatusAdDetection(mainMP3File string, successful bool, status, model, errMsg string) {
	_ = updateEpisodeStatus(mainMP3File, func(st *EpisodeStatusFile) {
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
	if !fileExists(jsonFile) {
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

func handleRecut(mainMP3File, sourceAudioFile, precutFile, outputFile, baseName string, totalDuration float64, selectedProfile LLMProfile, cfg Config, cli CLIOptions, fileStartTime time.Time) {
	pipeline.HandleRecut(mainMP3File, sourceAudioFile, precutFile, outputFile, baseName, totalDuration, selectedProfile, cfg, cli, fileStartTime)
}

func handleTranscribeMin(sourceAudioFile *string, totalDuration float64, cli CLIOptions) float64 {
	return pipeline.HandleTranscribeMin(sourceAudioFile, totalDuration, cli.TranscribeMin)
}

func formatTranscript(data *TranscriptionData, totalDuration float64) string {
	return pipeline.FormatTranscript(data, totalDuration)
}

func loadOrTranscribe(sourceAudioFile, jsonFile string, cfg Config, cli CLIOptions, selectedProfile LLMProfile, totalDuration, speedFactor float64, whisperLanguage, whisperPrompt string, id3TagsOut map[string]string, isNewlyTranscribed *bool, t0Step1 *time.Time) (*TranscriptionData, error) {
	return pipeline.LoadOrTranscribe(sourceAudioFile, jsonFile, cfg, cli, selectedProfile, totalDuration, speedFactor, whisperLanguage, whisperPrompt, id3TagsOut, isNewlyTranscribed, t0Step1)
}

func processJSONFile(inputFile string, cli CLIOptions) {
	pipeline.ProcessJSONFile(inputFile, cli)
}

func saveJSONTranscript(mainFile string, data *TranscriptionData, jsonFile string, quiet bool, id3Tags map[string]string) error {
	return pipeline.SaveJSONTranscript(mainFile, data, jsonFile, quiet, id3Tags)
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

func step1Duration(t0 time.Time) time.Duration { return time.Since(t0) }

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

func filterMP3FilesByPodcastConfig(files []string, dir string, cfg PodcastConfig) []string {
	if len(files) == 0 {
		return files
	}
	mode := normalizeAdRemovalMode(cfg.AdRemoval)
	if mode == AdRemovalNone {
		return nil
	}
	if mode == AdRemovalAll {
		return files
	}
	type fileWithTime struct {
		path    string
		pubTime time.Time
	}
	var list []fileWithTime
	for _, f := range files {
		pt := getEpisodePublicationTime(f)
		list = append(list, fileWithTime{path: f, pubTime: pt})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].pubTime.Equal(list[j].pubTime) {
			return list[i].path < list[j].path
		}
		return list[i].pubTime.After(list[j].pubTime)
	})
	for _, item := range list {
		base := strings.TrimSuffix(item.path, ".mp3")
		if _, err := os.Stat(base + ".cuts.json"); err != nil {
			return []string{item.path}
		}
	}
	return []string{list[0].path}
}

func updateQueue(dir string, mutate func([]string) []string) error {
	queueUpdateMu.Lock()
	defer queueUpdateMu.Unlock()

	path := filepath.Join(dir, "queue.json")
	lock, err := util.AcquireFileLockWithTimeout(path, 5*time.Second)
	if err != nil || lock == nil {
		return fmt.Errorf("queue is locked: %w", err)
	}
	defer lock.Release()

	var entries []string
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &entries); err != nil {
			return err
		}
	}
	entries = mutate(entries)
	if entries == nil {
		entries = []string{}
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return util.WriteFileAtomic(path, append(data, '\n'), 0644)
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

func getRemoteTransport() remote.RemoteTransport {
	return remote.GetRemoteTransport()
}

func isRemoteHostReachable(host string, transport remote.RemoteTransport) bool {
	return remote.IsRemoteHostReachable(host, transport)
}

func loadDoneManifest(path string) (*remote.RemoteDoneManifest, error) {
	return remote.LoadDoneManifest(path)
}

func formatPlayerTime(sec float64) string { return player.FormatPlayerTime(sec) }

func runPlayerDaemon(audioPath, title, podcast string) error {
	return player.RunPlayerDaemon(audioPath, title, podcast)
}

func isPlayerSocketAlive() bool { return player.IsPlayerSocketAlive() }

func startPlayerTrack(audioPath, title, podcast string) error {
	return player.StartPlayerTrack(audioPath, title, podcast)
}

func StartPlayerTrack(audioPath, title, podcast string) error {
	return player.StartPlayerTrack(audioPath, title, podcast)
}

func PausePlayerSocket() (bool, error) { return player.PausePlayerSocket() }
func ResumePlayerSocket() error        { return player.ResumePlayerSocket() }
func StopPlayerSocket() error          { return player.StopPlayerSocket() }
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

func isAudiobookshelfActive(cfg Config) bool { return backend.IsAudiobookshelfActive(&cfg) }
func isPodfetchActive(cfg Config) bool       { return backend.IsPodfetchActive(&cfg) }

func detectAdsLLM(transcriptText string, profile LLMProfile) ([]AdSegment, error) {
	return detect.DetectAdsLLM(transcriptText, profile, profile.APIKey)
}

func isGeminiEngine(cfg Config, cli CLIOptions) bool {
	if cli.WhisperEngine == string(WhisperEngineGemini) {
		return true
	}
	wp := config.GetActiveWhisperProfile(&cfg)
	return wp.Engine == WhisperEngineGemini
}

func sortAudioFilesByDuration(files []string) {
	remote.SortAudioFilesByDuration(files)
}

func getActiveWhisperProfile(cfg Config) types.WhisperProfile {
	return config.GetActiveWhisperProfile(&cfg)
}

func normalizeWhisperProfile(p types.WhisperProfile) types.WhisperProfile {
	return config.NormalizeWhisperProfile(p)
}

func whisperEngineBadge(engine types.WhisperEngine) string {
	return config.WhisperEngineBadge(engine)
}

func extractMetadataPrompt(sourceAudioFile string, id3TagsOut map[string]string, selectedProfile LLMProfile, cli CLIOptions) string {
	return pipeline.ExtractMetadataPrompt(sourceAudioFile, id3TagsOut, selectedProfile, cli)
}

func detectWhisperDockerContainer(whisperURL string) string {
	return transcribe.DetectWhisperDockerContainer(whisperURL)
}

func runWhisperCLITranscriptionContext(ctx context.Context, audioPath string, profile types.WhisperProfile, quiet, verbose bool, prompt, lang string) (*types.TranscriptionData, error) {
	return transcribe.RunWhisperCLITranscriptionContext(ctx, audioPath, profile, quiet, verbose, prompt, lang)
}

func transcribeChunks(audioPath, whisperURL string, quiet, verbose bool, totalDuration, speedFactor float64, chunkDuration int, dockerContainer string, prompt, language string) (*types.TranscriptionData, error) {
	return transcribe.TranscribeChunks(audioPath, whisperURL, quiet, verbose, totalDuration, speedFactor, chunkDuration, dockerContainer, prompt, language)
}

func transcribeWhisperContext(ctx context.Context, audioPath, whisperURL string, quiet, verbose bool, totalDuration, speedFactor float64, dockerContainer string, prompt, language string, pcmData []byte) (*types.TranscriptionData, error) {
	return transcribe.TranscribeWhisperContext(ctx, audioPath, whisperURL, quiet, verbose, totalDuration, speedFactor, dockerContainer, prompt, language, pcmData)
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


func syncAudiobookshelfDuration(cfg *Config, filePath string, duration float64) {
	remote.SyncAudiobookshelfDuration(cfg, filePath, duration)
}

func getPubMS(ep FeedEpisode) int64 {
	return podcast.GetPubMS(ep)
}

func findPodcastDirForItem(item backend.Podcast, podcastsDir string) string {
	return podcast.FindPodcastDirForItem(item, podcastsDir)
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


func defaultPodcastConfig() PodcastConfig {
	return config.DefaultPodcastConfig(nil)
}

func addDoneEpisode(manifestPath string, item RemoteDoneItem) error {
	return remote.AddDoneEpisode(manifestPath, item)
}

func setRemoteTransport(t remote.RemoteTransport) {
	remote.SetRemoteTransport(t)
}

