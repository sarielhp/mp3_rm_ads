package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type MockRemoteTransport struct {
	RemoteRoot   string
	Reachable    bool
	ExecutedCmds []string
	ExecHandler  func(host, cmd string) (string, error)
}

func NewMockRemoteTransport(root string) *MockRemoteTransport {
	return &MockRemoteTransport{
		RemoteRoot: root,
		Reachable:  true,
	}
}

func (m *MockRemoteTransport) resolveRemotePath(remPath string) string {
	if strings.HasPrefix(remPath, m.RemoteRoot) {
		return remPath
	}
	clean := strings.TrimPrefix(remPath, "~/")
	clean = strings.TrimPrefix(clean, ".abs_remote/")
	clean = strings.TrimPrefix(clean, "abs_remote/")
	clean = strings.TrimPrefix(clean, ".config/")
	clean = strings.TrimPrefix(clean, ".local/")
	return filepath.Join(m.RemoteRoot, clean)
}

func (m *MockRemoteTransport) Exec(host string, cmd string) (string, error) {
	m.ExecutedCmds = append(m.ExecutedCmds, cmd)
	if !m.Reachable {
		return "", fmt.Errorf("host %s unreachable", host)
	}
	if m.ExecHandler != nil {
		return m.ExecHandler(host, cmd)
	}

	if strings.Contains(cmd, "echo 1") || strings.Contains(cmd, "echo ping") {
		return "1", nil
	}
	if strings.HasPrefix(cmd, "mkdir -p") {
		raw := strings.TrimPrefix(cmd, "mkdir -p")
		raw = strings.Trim(strings.TrimSpace(raw), "\"")
		local := m.resolveRemotePath(raw)
		_ = os.MkdirAll(local, 0755)
		return "", nil
	}
	if strings.HasPrefix(cmd, "ls -1") {
		parts := strings.Fields(cmd)
		if len(parts) >= 3 {
			dir := m.resolveRemotePath(parts[2])
			entries, err := os.ReadDir(dir)
			if err != nil {
				return "", nil
			}
			var names []string
			for _, e := range entries {
				names = append(names, e.Name())
			}
			return strings.Join(names, "\n"), nil
		}
		return "", nil
	}
	if strings.HasPrefix(cmd, "rm -rf") {
		parts := strings.Fields(cmd)
		if len(parts) >= 3 {
			target := m.resolveRemotePath(parts[2])
			_ = os.RemoveAll(target)
		}
		return "", nil
	}
	if strings.HasPrefix(cmd, "touch ") {
		parts := strings.Fields(cmd)[1:]
		for _, p := range parts {
			local := m.resolveRemotePath(p)
			_ = os.WriteFile(local, []byte{}, 0644)
		}
		return "", nil
	}
	if strings.Contains(cmd, "remote ack") {
		var rels []string
		parts := strings.Split(cmd, "\"")
		for i := 1; i < len(parts); i += 2 {
			if strings.TrimSpace(parts[i]) != "" {
				rels = append(rels, strings.TrimSpace(parts[i]))
			}
		}
		if len(rels) == 0 {
			fields := strings.Fields(cmd)
			for i, f := range fields {
				if f == "ack" && i+1 < len(fields) {
					rels = append(rels, fields[i+1:]...)
					break
				}
			}
		}
		dir := m.RemoteRoot
		if len(rels) > 0 && fileExists(filepath.Join(m.RemoteRoot, "remote_root", rels[0])) {
			dir = filepath.Join(m.RemoteRoot, "remote_root")
		}
		_ = runRemoteAck(dir, rels)
		return "", nil
	}
	if strings.Contains(cmd, "abs help") || strings.Contains(cmd, "abs version") {
		return "abs version 0.1.26", nil
	}
	if strings.HasPrefix(cmd, "pgrep") {
		return "12345\n", nil
	}

	return "", nil
}

func (m *MockRemoteTransport) Upload(host string, localSrc, remoteDst string) error {
	if !m.Reachable {
		return fmt.Errorf("host %s unreachable", host)
	}
	dst := m.resolveRemotePath(remoteDst)
	_ = os.MkdirAll(filepath.Dir(dst), 0755)
	return copyFileOrDir(localSrc, dst)
}

func (m *MockRemoteTransport) Download(host string, remoteSrc, localDst string) error {
	if !m.Reachable {
		return fmt.Errorf("host %s unreachable", host)
	}
	src := m.resolveRemotePath(remoteSrc)
	_ = os.MkdirAll(filepath.Dir(localDst), 0755)
	return copyFileOrDir(src, localDst)
}

func (m *MockRemoteTransport) RsyncTo(host string, localSrc, remoteDst string) error {
	return m.Upload(host, localSrc, remoteDst)
}

func (m *MockRemoteTransport) RsyncFrom(host string, remoteSrc, localDst string) error {
	return m.Download(host, remoteSrc, localDst)
}

func copyFileOrDir(src, dst string) error {
	fi, err := os.Stat(src)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		_ = os.MkdirAll(dst, 0755)
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			srcChild := filepath.Join(src, e.Name())
			dstChild := filepath.Join(dst, e.Name())
			if err := copyFileOrDir(srcChild, dstChild); err != nil {
				return err
			}
		}
		return nil
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	_ = os.MkdirAll(filepath.Dir(dst), 0755)
	return os.WriteFile(dst, data, fi.Mode())
}

func TestRemoteConfigOptions(t *testing.T) {
	tempDir := t.TempDir()
	testConfigPath = filepath.Join(tempDir, "config.json")
	defer func() { testConfigPath = "" }()

	cfg := loadConfig()
	if cfg.DefaultProcessing != "local" {
		t.Errorf("expected default processing to be 'local', got %s", cfg.DefaultProcessing)
	}
	if cfg.RemoteWorkDir != "~/abs_remote" {
		t.Errorf("expected default remote work dir to be '~/abs_remote', got %s", cfg.RemoteWorkDir)
	}

	if err := handleConfigSet(&cfg, "remote-host", "server1.lan"); err != nil {
		t.Fatalf("handleConfigSet remote-host failed: %v", err)
	}
	if cfg.RemoteHost != "server1.lan" {
		t.Errorf("expected RemoteHost server1.lan, got %s", cfg.RemoteHost)
	}

	if err := handleConfigSet(&cfg, "default-processing", "remote"); err != nil {
		t.Fatalf("handleConfigSet default-processing failed: %v", err)
	}
	if cfg.DefaultProcessing != "remote" {
		t.Errorf("expected DefaultProcessing remote, got %s", cfg.DefaultProcessing)
	}

	if err := handleConfigSet(&cfg, "default-processing", "invalid-mode"); err == nil {
		t.Errorf("expected error setting invalid default-processing value, got nil")
	}

	if err := handleConfigSet(&cfg, "remote-work-dir", "/data/abs_work"); err != nil {
		t.Fatalf("handleConfigSet remote-work-dir failed: %v", err)
	}
	if cfg.RemoteWorkDir != "/data/abs_work" {
		t.Errorf("expected RemoteWorkDir /data/abs_work, got %s", cfg.RemoteWorkDir)
	}
}

func TestRemoteTypesAndManifest(t *testing.T) {
	tempDir := t.TempDir()
	manifestPath := filepath.Join(tempDir, "manifest.json")

	batchID := generateBatchID()
	manifest := RemoteBatchManifest{
		BatchID:    batchID,
		CreatedAt:  "2026-08-30T12:00:00Z",
		Host:       "remote-server",
		Status:     BatchStatusQueued,
		TotalItems: 2,
		Items: []RemoteBatchJobItem{
			{
				ID:            batchID + "-1",
				SourceFile:    "/podcasts/ep1.mp3",
				AudioFileName: "ep1.mp3",
				Status:        BatchStatusQueued,
			},
			{
				ID:            batchID + "-2",
				SourceFile:    "/podcasts/ep2.mp3",
				AudioFileName: "ep2.mp3",
				Status:        BatchStatusQueued,
			},
		},
	}

	if err := saveManifest(manifestPath, &manifest); err != nil {
		t.Fatalf("saveManifest failed: %v", err)
	}

	loaded, err := loadManifest(manifestPath)
	if err != nil {
		t.Fatalf("loadManifest failed: %v", err)
	}
	if loaded.BatchID != batchID || loaded.TotalItems != 2 {
		t.Errorf("unexpected loaded manifest: %+v", loaded)
	}

	updateManifestItem(loaded, batchID+"-1", BatchStatusCompleted, "")
	updateManifestItem(loaded, batchID+"-2", BatchStatusFailed, "ffmpeg error")

	if loaded.CompletedItems != 1 || loaded.FailedItems != 1 {
		t.Errorf("expected 1 completed and 1 failed item, got %d and %d", loaded.CompletedItems, loaded.FailedItems)
	}
	if loaded.Status != BatchStatusCompleted {
		t.Errorf("expected overall batch status completed, got %s", loaded.Status)
	}
}

func TestResolveProcessingHostFallback(t *testing.T) {
	tempDir := t.TempDir()
	mock := NewMockRemoteTransport(tempDir)

	cfg := &Config{
		RemoteHost:        "remote-box",
		DefaultProcessing: "remote",
	}

	host, isRemote, err := ResolveProcessingHost(cfg, "", mock)
	if err != nil || !isRemote || host != "remote-box" {
		t.Errorf("expected remote-box, true; got %s, %v, %v", host, isRemote, err)
	}

	mock.Reachable = false
	hostFallback, isRemoteFallback, errFallback := ResolveProcessingHost(cfg, "", mock)
	if errFallback != nil || isRemoteFallback || hostFallback != "local" {
		t.Errorf("expected local fallback, false; got %s, %v, %v", hostFallback, isRemoteFallback, errFallback)
	}

	explicitHost, explicitRemote, _ := ResolveProcessingHost(cfg, "gpu-cluster", mock)
	if explicitHost != "gpu-cluster" || !explicitRemote {
		t.Errorf("expected gpu-cluster, true; got %s, %v", explicitHost, explicitRemote)
	}
}

func TestRemoteDeploy(t *testing.T) {
	tempDir := t.TempDir()
	mock := NewMockRemoteTransport(tempDir)

	cfg := &Config{
		RemoteHost:    "deploy-box",
		RemoteWorkDir: "~/.abs_remote",
	}

	testConfigPath = filepath.Join(tempDir, "local_config.json")
	defer func() { testConfigPath = "" }()
	saveConfig(*cfg)

	err := runRemoteDeploy(cfg, "", mock, true, false)
	if err != nil {
		t.Fatalf("runRemoteDeploy failed: %v", err)
	}

	noHostCfg := &Config{}
	if err := runRemoteDeploy(noHostCfg, "", mock, true, false); err == nil {
		t.Errorf("expected error when no host specified for deploy, got nil")
	}
}

func TestRemotePush(t *testing.T) {
	tempDir := t.TempDir()
	mock := NewMockRemoteTransport(tempDir)

	podcastsDir := filepath.Join(tempDir, "podcasts")
	_ = os.MkdirAll(podcastsDir, 0755)
	ep1 := filepath.Join(podcastsDir, "test1.mp3")
	ep2 := filepath.Join(podcastsDir, "test2.mp3")
	_ = os.WriteFile(ep1, []byte("fake audio 1"), 0644)
	_ = os.WriteFile(ep2, []byte("fake audio 2"), 0644)

	cfg := &Config{
		RemoteHost:    "push-box",
		RemoteWorkDir: "~/.abs_remote",
		PodcastsDir:   podcastsDir,
	}

	err := runRemotePush(cfg, []string{ep1, ep2}, "push-box", mock, true, false)
	if err != nil {
		t.Fatalf("runRemotePush failed: %v", err)
	}

	if len(mock.ExecutedCmds) == 0 {
		t.Errorf("expected commands to be executed on mock transport")
	}
}

func TestBatchWorkerExecution(t *testing.T) {
	tempDir := t.TempDir()
	batchDir := filepath.Join(tempDir, "batch_work")
	inDir := filepath.Join(batchDir, "in")
	outDir := filepath.Join(batchDir, "out")
	_ = os.MkdirAll(inDir, 0755)
	_ = os.MkdirAll(outDir, 0755)

	epFile := filepath.Join(inDir, "episode1.mp3")
	_ = os.WriteFile(epFile, []byte("fake mp3 stream content"), 0644)

	manifest := RemoteBatchManifest{
		BatchID:    "test-batch-001",
		CreatedAt:  "2026-08-30T12:00:00Z",
		Status:     BatchStatusQueued,
		TotalItems: 1,
		Items: []RemoteBatchJobItem{
			{
				ID:            "test-batch-001-1",
				SourceFile:    "/origin/episode1.mp3",
				AudioFileName: "episode1.mp3",
				Status:        BatchStatusQueued,
			},
		},
	}
	_ = saveManifest(filepath.Join(batchDir, "manifest.json"), &manifest)

	dummyTranscript := TranscriptionData{
		Text: "Hello and welcome to the show.",
		Segments: []TranscriptionSegment{
			{Start: 0.0, End: 5.0, Text: "Hello and welcome to the show."},
		},
	}
	dummyTranscriptPath := filepath.Join(outDir, "episode1.transcript.json")
	saveJSONTranscript(filepath.Join(outDir, "episode1.mp3"), &dummyTranscript, dummyTranscriptPath, true, nil)

	err := runBatchWorker(batchDir, true, false)
	if err != nil {
		t.Fatalf("runBatchWorker failed: %v", err)
	}

	updated, err := loadManifest(filepath.Join(batchDir, "manifest.json"))
	if err != nil {
		t.Fatalf("failed to reload manifest: %v", err)
	}
	if updated.Status != BatchStatusCompleted {
		t.Errorf("expected batch status completed, got %s", updated.Status)
	}
}

func TestRemotePull(t *testing.T) {
	tempDir := t.TempDir()
	mock := NewMockRemoteTransport(tempDir)

	localPodcasts := filepath.Join(tempDir, "local_podcasts")
	_ = os.MkdirAll(localPodcasts, 0755)
	localEp := filepath.Join(localPodcasts, "ep_pull.mp3")
	_ = os.WriteFile(localEp, []byte("original ep content"), 0644)

	remoteBatchDir := filepath.Join(tempDir, "staging", "batch-pull-test")
	remoteOutDir := filepath.Join(remoteBatchDir, "out")
	_ = os.MkdirAll(remoteOutDir, 0755)

	cleanedAudio := filepath.Join(remoteOutDir, "ep_pull.mp3")
	cutsJSON := filepath.Join(remoteOutDir, "ep_pull.cuts.json")
	transJSON := filepath.Join(remoteOutDir, "ep_pull.transcript.json")
	_ = os.WriteFile(cleanedAudio, []byte("cleaned ad-free audio"), 0644)
	_ = os.WriteFile(cutsJSON, []byte(`{"cut_intervals":[]}`), 0644)
	_ = os.WriteFile(transJSON, []byte(`{"text":"transcript"}`), 0644)

	manifest := RemoteBatchManifest{
		BatchID:        "batch-pull-test",
		CreatedAt:      "2026-08-30T12:00:00Z",
		Status:         BatchStatusCompleted,
		TotalItems:     1,
		CompletedItems: 1,
		Items: []RemoteBatchJobItem{
			{
				ID:                  "batch-pull-test-1",
				SourceFile:          localEp,
				AudioFileName:       "ep_pull.mp3",
				Status:              BatchStatusCompleted,
				CleanedAudioFile:    "ep_pull.mp3",
				CutsJSONFile:        "ep_pull.cuts.json",
				TranscriptJSONFile:  "ep_pull.transcript.json",
				OriginalDurationSec: 100.0,
				CleanedDurationSec:  80.0,
				CutDurationSec:      20.0,
			},
		},
	}
	_ = saveManifest(filepath.Join(remoteBatchDir, "manifest.json"), &manifest)

	cfg := &Config{
		RemoteHost:    "pull-box",
		RemoteWorkDir: "~/.abs_remote",
		PodcastsDir:   localPodcasts,
	}

	err := runRemotePull(cfg, "pull-box", mock, true, false)
	if err != nil {
		t.Fatalf("runRemotePull failed: %v", err)
	}

	data, err := os.ReadFile(localEp)
	if err != nil || string(data) != "cleaned ad-free audio" {
		t.Errorf("expected cleaned audio pulled into local file, got: %s (err: %v)", string(data), err)
	}
	if !fileExists(localEp + ".precut") {
		t.Errorf("expected precut backup file to exist")
	}
}

func TestRemoteStatusAndCancel(t *testing.T) {
	tempDir := t.TempDir()
	mock := NewMockRemoteTransport(tempDir)

	remoteBatchDir := filepath.Join(tempDir, "staging", "batch-cancel-test")
	_ = os.MkdirAll(remoteBatchDir, 0755)

	manifest := RemoteBatchManifest{
		BatchID:    "batch-cancel-test",
		CreatedAt:  "2026-08-30T12:00:00Z",
		Status:     BatchStatusProcessing,
		TotalItems: 1,
		Items: []RemoteBatchJobItem{
			{
				ID:            "batch-cancel-test-1",
				AudioFileName: "job.mp3",
				Status:        BatchStatusProcessing,
			},
		},
	}
	_ = saveManifest(filepath.Join(remoteBatchDir, "manifest.json"), &manifest)

	cfg := &Config{
		RemoteHost:    "status-box",
		RemoteWorkDir: "~/.abs_remote",
	}

	if err := runRemoteStatus(cfg, "status-box", mock, true, true); err != nil {
		t.Fatalf("runRemoteStatus failed: %v", err)
	}

	if err := runRemoteCancel(cfg, "status-box", "batch-cancel-test", mock, true); err != nil {
		t.Fatalf("runRemoteCancel with batch ID failed: %v", err)
	}

	if err := runRemoteCancel(cfg, "status-box", "", mock, true); err != nil {
		t.Fatalf("runRemoteCancel all failed: %v", err)
	}
}
