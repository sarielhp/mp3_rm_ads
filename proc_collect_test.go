package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIProcCollectAndNoCollectParsing(t *testing.T) {
	var action string
	var opts CLIOptions
	app := buildCLIApp(&action, &opts)

	if err := app.Execute([]string{"proc", "collect"}); err != nil {
		t.Fatalf("failed to parse 'proc collect': %v", err)
	}
	if action != "proc" || opts.ProcSubcmd != "collect" {
		t.Errorf("expected action=proc, ProcSubcmd=collect, got action=%s, ProcSubcmd=%s", action, opts.ProcSubcmd)
	}

	action = ""
	opts = CLIOptions{}
	app = buildCLIApp(&action, &opts)
	if err := app.Execute([]string{"proc", "collect", "mybox", "-q", "-v"}); err != nil {
		t.Fatalf("failed to parse 'proc collect mybox -q -v': %v", err)
	}
	if action != "proc" || opts.ProcSubcmd != "collect" || opts.RemoteHost != "mybox" || !opts.Quiet || !opts.Verbose {
		t.Errorf("expected proc collect mybox -q -v, got action=%s, subcmd=%s, host=%s, quiet=%v, verbose=%v",
			action, opts.ProcSubcmd, opts.RemoteHost, opts.Quiet, opts.Verbose)
	}

	action = ""
	opts = CLIOptions{}
	app = buildCLIApp(&action, &opts)
	if err := app.Execute([]string{"proc", "--no-collect"}); err != nil {
		t.Fatalf("failed to parse 'proc --no-collect': %v", err)
	}
	if action != "proc" || !opts.NoCollect {
		t.Errorf("expected action=proc, NoCollect=true, got action=%s, NoCollect=%v", action, opts.NoCollect)
	}
}

func TestProcCollectExecution(t *testing.T) {
	tempDir := t.TempDir()
	mock := NewMockRemoteTransport(tempDir)
	setRemoteTransport(mock)
	t.Cleanup(func() { setRemoteTransport(&DefaultSSHTransport{}) })

	localPodcasts := filepath.Join(tempDir, "local_podcasts")
	localPodDir := filepath.Join(localPodcasts, "Science_Pod")
	_ = os.MkdirAll(localPodDir, 0755)
	localAudio := filepath.Join(localPodDir, "ep1.mp3")
	_ = os.WriteFile(localAudio, []byte("original raw local audio"), 0644)

	remoteWorkDir := filepath.Join(tempDir, "remote_root")
	remotePodDir := filepath.Join(remoteWorkDir, "Science_Pod")
	_ = os.MkdirAll(remotePodDir, 0755)

	remAudio := filepath.Join(remotePodDir, "ep1.mp3")
	remCuts := filepath.Join(remotePodDir, "ep1.cuts.json")
	remTrans := filepath.Join(remotePodDir, "ep1.transcript.json")
	remStat := filepath.Join(remotePodDir, "ep1.mp3.json")

	_ = os.WriteFile(remAudio, []byte("cleaned remote audio"), 0644)
	_ = os.WriteFile(remCuts, []byte(`{"cut_intervals":[{"start_sec":10,"end_sec":20}]}`), 0644)
	_ = os.WriteFile(remTrans, []byte(`{"text":"cleaned transcript"}`), 0644)
	_ = saveEpisodeStatus(remStat, &EpisodeStatusFile{
		Version:   1,
		MediaFile: "ep1.mp3",
		Status:    StateReadyForCopyBack,
	})

	relPath := filepath.Join("Science_Pod", "ep1.mp3")
	donePath := filepath.Join(remoteWorkDir, "done.json")
	_ = addDoneEpisode(donePath, RemoteDoneItem{
		RelPath:            relPath,
		Status:             StateReadyForCopyBack,
		CleanedDurationSec: 100.0,
		CutDurationSec:     10.0,
	})

	cfg := Config{
		RemoteHost:    "mock-host",
		RemoteWorkDir: remoteWorkDir,
		PodcastsDir:   localPodcasts,
	}

	cli := CLIOptions{
		ProcSubcmd: "collect",
		RemoteHost: "mock-host",
		Quiet:      true,
	}

	processAudioFilesBatch(cli, cfg, "proc")

	pulledData, err := os.ReadFile(localAudio)
	if err != nil || string(pulledData) != "cleaned remote audio" {
		t.Fatalf("expected pulled audio content 'cleaned remote audio', got: %s (err: %v)", string(pulledData), err)
	}

	localPrecut := localAudio + ".precut"
	precutData, err := os.ReadFile(localPrecut)
	if err != nil || string(precutData) != "original raw local audio" {
		t.Errorf("expected local precut backup, got: %s (err: %v)", string(precutData), err)
	}

	st, err := loadEpisodeStatus(statusPathFor(localAudio))
	if err != nil || st.Status != StateDone {
		t.Errorf("expected local episode status StateDone, got %+v (err: %v)", st, err)
	}

	if !fileExists(filepath.Join(localPodDir, "ep1.cuts.json")) {
		t.Errorf("expected cuts.json to exist locally")
	}
	if !fileExists(filepath.Join(localPodDir, "ep1.transcript.json")) {
		t.Errorf("expected transcript.json to exist locally")
	}

	remFi, err := os.Stat(remAudio)
	if err != nil || remFi.Size() != 0 {
		t.Errorf("expected remote audio to be truncated to 0 bytes, got size %d", remFi.Size())
	}
}

func TestAutoCollectionBeforePushInBatchProc(t *testing.T) {
	tempDir := t.TempDir()
	mock := NewMockRemoteTransport(tempDir)
	setRemoteTransport(mock)
	t.Cleanup(func() { setRemoteTransport(&DefaultSSHTransport{}) })

	localPodcasts := filepath.Join(tempDir, "local_podcasts")
	localPodDir := filepath.Join(localPodcasts, "Daily_News")
	_ = os.MkdirAll(localPodDir, 0755)

	doneAudio := filepath.Join(localPodDir, "ep_done.mp3")
	_ = os.WriteFile(doneAudio, []byte("raw done ep audio"), 0644)

	newAudio := filepath.Join(localPodDir, "ep_new.mp3")
	_ = os.WriteFile(newAudio, []byte("raw new ep audio"), 0644)

	remoteWorkDir := filepath.Join(tempDir, "remote_root")
	remotePodDir := filepath.Join(remoteWorkDir, "Daily_News")
	_ = os.MkdirAll(remotePodDir, 0755)

	remDoneAudio := filepath.Join(remotePodDir, "ep_done.mp3")
	remDoneCuts := filepath.Join(remotePodDir, "ep_done.cuts.json")
	remDoneTrans := filepath.Join(remotePodDir, "ep_done.transcript.json")
	remDoneStat := filepath.Join(remotePodDir, "ep_done.mp3.json")

	_ = os.WriteFile(remDoneAudio, []byte("cleaned done ep audio"), 0644)
	_ = os.WriteFile(remDoneCuts, []byte(`{"cut_intervals":[]}`), 0644)
	_ = os.WriteFile(remDoneTrans, []byte(`{"text":"done transcript"}`), 0644)
	_ = saveEpisodeStatus(remDoneStat, &EpisodeStatusFile{
		Version:   1,
		MediaFile: "ep_done.mp3",
		Status:    StateReadyForCopyBack,
	})

	donePath := filepath.Join(remoteWorkDir, "done.json")
	_ = addDoneEpisode(donePath, RemoteDoneItem{
		RelPath:            filepath.Join("Daily_News", "ep_done.mp3"),
		Status:             StateReadyForCopyBack,
		CleanedDurationSec: 60.0,
	})

	cfg := Config{
		RemoteHost:    "mock-host",
		RemoteWorkDir: remoteWorkDir,
		PodcastsDir:   localPodcasts,
	}

	cli := CLIOptions{
		Remote: true,
		Quiet:  true,
	}

	processAudioFilesBatch(cli, cfg, "proc")

	doneData, err := os.ReadFile(doneAudio)
	if err != nil || string(doneData) != "cleaned done ep audio" {
		t.Errorf("expected ep_done.mp3 to be auto-collected before push, got: %s", string(doneData))
	}

	stDone, err := loadEpisodeStatus(statusPathFor(doneAudio))
	if err != nil || stDone.Status != StateDone {
		t.Errorf("expected ep_done.mp3 status StateDone, got %+v", stDone)
	}

	remPushedAudio := filepath.Join(remotePodDir, "ep_new.mp3")
	if !fileExists(remPushedAudio) {
		t.Errorf("expected ep_new.mp3 to be pushed to remote mirror at %s", remPushedAudio)
	}

	stNew, err := loadEpisodeStatus(statusPathFor(newAudio))
	if err != nil || stNew.Status != StateQueuedRemote {
		t.Errorf("expected ep_new.mp3 local status StateQueuedRemote, got %+v (err: %v)", stNew, err)
	}
}

func TestAutoCollectionSkippedWithNoCollectFlag(t *testing.T) {
	tempDir := t.TempDir()
	mock := NewMockRemoteTransport(tempDir)
	setRemoteTransport(mock)
	t.Cleanup(func() { setRemoteTransport(&DefaultSSHTransport{}) })

	localPodcasts := filepath.Join(tempDir, "local_podcasts")
	localPodDir := filepath.Join(localPodcasts, "Pod")
	_ = os.MkdirAll(localPodDir, 0755)

	doneAudio := filepath.Join(localPodDir, "ep.mp3")
	_ = os.WriteFile(doneAudio, []byte("raw local uncollected audio"), 0644)

	remoteWorkDir := filepath.Join(tempDir, "remote_root")
	remotePodDir := filepath.Join(remoteWorkDir, "Pod")
	_ = os.MkdirAll(remotePodDir, 0755)

	remAudio := filepath.Join(remotePodDir, "ep.mp3")
	_ = os.WriteFile(remAudio, []byte("cleaned remote audio"), 0644)

	donePath := filepath.Join(remoteWorkDir, "done.json")
	_ = addDoneEpisode(donePath, RemoteDoneItem{
		RelPath:            filepath.Join("Pod", "ep.mp3"),
		Status:             StateReadyForCopyBack,
		CleanedDurationSec: 60.0,
	})

	cfg := Config{
		RemoteHost:    "mock-host",
		RemoteWorkDir: remoteWorkDir,
		PodcastsDir:   localPodcasts,
	}

	cli := CLIOptions{
		Remote:    true,
		NoCollect: true,
		Quiet:     true,
	}

	processAudioFilesBatch(cli, cfg, "proc")

	data, err := os.ReadFile(doneAudio)
	if err != nil || string(data) != "raw local uncollected audio" {
		t.Errorf("expected collection to be skipped when NoCollect is true, got: %s", string(data))
	}
}

func TestProcDryRunRemoteCollectionCount(t *testing.T) {
	tempDir := t.TempDir()
	mock := NewMockRemoteTransport(tempDir)
	setRemoteTransport(mock)
	t.Cleanup(func() { setRemoteTransport(&DefaultSSHTransport{}) })

	remoteWorkDir := filepath.Join(tempDir, "remote_root")
	_ = os.MkdirAll(remoteWorkDir, 0755)

	donePath := filepath.Join(remoteWorkDir, "done.json")
	_ = addDoneEpisode(donePath, RemoteDoneItem{
		RelPath: filepath.Join("Pod", "ep1.mp3"),
		Status:  StateReadyForCopyBack,
	})
	_ = addDoneEpisode(donePath, RemoteDoneItem{
		RelPath: filepath.Join("Pod", "ep2.mp3"),
		Status:  StateReadyForCopyBack,
	})

	localPodcasts := filepath.Join(tempDir, "local_podcasts")
	_ = os.MkdirAll(localPodcasts, 0755)
	audio1 := filepath.Join(localPodcasts, "local1.mp3")
	_ = os.WriteFile(audio1, []byte("audio"), 0644)

	cfg := Config{
		RemoteHost:    "mock-host",
		RemoteWorkDir: remoteWorkDir,
		PodcastsDir:   localPodcasts,
	}

	cli := CLIOptions{
		Remote: true,
		DryRun: true,
	}

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	handleProcDryRun([]string{audio1}, cli, cfg)

	_ = w.Close()
	os.Stdout = oldStdout

	outBytes, _ := io.ReadAll(r)
	out := string(outBytes)

	if !strings.Contains(out, "Ready for Remote Collection:   2 (mock-host)") {
		t.Errorf("expected dry run output to mention 2 ready for remote collection, got: %s", out)
	}
}
