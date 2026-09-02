package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type dryRunFileStatus struct {
	path   string
	status string
}

func auditFileStatus(inputFile string, cli CLIOptions) (category string, statusText string) {
	mainMP3File, precutFile, _ := resolveAudioFiles(inputFile, cli)
	statFile := statusPathFor(mainMP3File)
	st, _ := loadEpisodeStatus(statFile)

	if st != nil {
		switch st.Status {
		case StateDone, StateCopiedBack, StateArchived:
			return "completed", "Completed (Ad-Free)"
		case StateReadyForCopyBack:
			return "remote_pending", "Ready for Pull (Remote Done)"
		case StateQueuedRemote, StateTranscribingRemotely, StateCuttingRemotely, StateAwaitingTranscription:
			return "remote_pending", fmt.Sprintf("Remote Processing (%s)", st.Status)
		}
	}

	baseName := stripExt(mainMP3File)
	jsonFile := cli.TranscriptPath
	if jsonFile == "" {
		jsonFile = baseName + ".transcript.json"
	}
	cutsFile := baseName + ".cuts.json"

	if !fileExists(jsonFile) {
		return "needs_tx", "Needs Transcription"
	}
	if !fileExists(cutsFile) {
		return "needs_llm", "Needs Ad Detection (LLM)"
	}
	if fileExists(precutFile) {
		return "completed", "Completed (Ad-Free)"
	}
	data, err := os.ReadFile(cutsFile)
	var cd CutsData
	if err == nil && json.Unmarshal(data, &cd) == nil && len(cd.CutIntervals) > 0 {
		return "needs_cut", "Needs Audio Cutting"
	}
	return "completed", "Completed (0 ads)"
}

func fetchRemoteReadyCount(cli CLIOptions, config Config) (int, string) {
	if cli.Local {
		return 0, ""
	}
	reqHost := ""
	if cli.Remote {
		reqHost = config.RemoteHost
		if reqHost == "" {
			reqHost = "cloud8"
		}
	}
	h, isRem, err := ResolveProcessingHost(&config, reqHost, nil)
	if err != nil || !isRem || h == "" {
		return 0, ""
	}
	transport := getRemoteTransport()
	if !isRemoteHostReachable(h, transport) {
		return 0, h
	}
	remoteWorkDir := "~/abs_remote"
	if config.RemoteWorkDir != "" {
		remoteWorkDir = config.RemoteWorkDir
	}
	tempDonePath := filepath.Join(os.TempDir(), fmt.Sprintf("dryrun_done_%d.json", time.Now().UnixNano()))
	remoteDoneFile := fmt.Sprintf("%s/done.json", remoteWorkDir)
	remoteReadyOnServer := 0
	if err := transport.Download(h, remoteDoneFile, tempDonePath); err == nil {
		if doneM, err := loadDoneManifest(tempDonePath); err == nil && doneM != nil {
			for _, it := range doneM.Episodes {
				if it.Status == StateReadyForCopyBack {
					remoteReadyOnServer++
				}
			}
		}
		_ = os.Remove(tempDonePath)
	}
	return remoteReadyOnServer, h
}

func printDryRunSummary(filesCount, needsTx, needsLLM, needsCut, remotePending, alreadyComplete, remoteReady int, targetHost string, cli CLIOptions, details []dryRunFileStatus) {
	totalNeedingAction := needsTx + needsLLM + needsCut
	fmt.Println()
	fmt.Println(bold("DRY RUN: Audio Processing Pipeline Status"))
	fmt.Println(strings.Repeat("─", 55))
	fmt.Printf("  • Total Episodes Scanned:        %d\n", filesCount)
	fmt.Printf("  • Needs Transcription (Whisper): %d\n", needsTx)
	fmt.Printf("  • Needs Ad Detection (LLM):      %d\n", needsLLM)
	fmt.Printf("  • Needs Audio Cutting (FFmpeg):  %d\n", needsCut)
	if remotePending > 0 {
		fmt.Printf("  • In Remote Queue / Ready:       %d\n", remotePending)
	}
	if targetHost != "" {
		fmt.Printf("  • Ready for Remote Collection:   %d (%s)\n", remoteReady, targetHost)
	}
	fmt.Printf("  • Already Processed / Ad-Free:   %d\n", alreadyComplete)
	fmt.Println(strings.Repeat("─", 55))
	fmt.Printf("  Total Needing Local Processing:  %s\n", bold(fmt.Sprintf("%d", totalNeedingAction)))
	if cli.Count > 0 && totalNeedingAction > cli.Count {
		fmt.Printf("  (Limit -n %d: would process first %d of %d episodes)\n", cli.Count, cli.Count, totalNeedingAction)
	}
	fmt.Println()

	if cli.Verbose {
		fmt.Println("Episode Details:")
		for _, d := range details {
			fmt.Printf("  [%s] %s\n", d.status, displayName(d.path))
		}
		fmt.Println()
	}
}

func handleProcDryRun(files []string, cli CLIOptions, config Config) {
	var needsTranscribe, needsLLM, needsCut, alreadyComplete, remotePending int
	var details []dryRunFileStatus

	for _, inputFile := range files {
		if strings.HasSuffix(inputFile, ".json") {
			continue
		}
		cat, desc := auditFileStatus(inputFile, cli)
		details = append(details, dryRunFileStatus{path: inputFile, status: desc})
		switch cat {
		case "completed":
			alreadyComplete++
		case "remote_pending":
			remotePending++
		case "needs_tx":
			needsTranscribe++
		case "needs_llm":
			needsLLM++
		case "needs_cut":
			needsCut++
		}
	}

	remoteReady, targetHost := fetchRemoteReadyCount(cli, config)
	printDryRunSummary(len(files), needsTranscribe, needsLLM, needsCut, remotePending, alreadyComplete, remoteReady, targetHost, cli, details)
}
