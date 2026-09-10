package adremoval

import (
	"abs/pkg/pipeline"
	"abs/pkg/remote"
	"abs/pkg/util"
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

func auditFileStatus(inputFile string, opts ProcOptions) (category string, statusText string) {
	mainMP3File, precutFile, _ := resolveAudioFiles(inputFile, opts)
	statFile := pipeline.StatusPathFor(mainMP3File)
	st, _ := pipeline.LoadEpisodeStatus(statFile)

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

	baseName := util.StripExt(mainMP3File)
	jsonFile := opts.TranscriptPath
	if jsonFile == "" {
		jsonFile = baseName + ".transcript.json"
	}
	cutsFile := baseName + ".cuts.json"

	if !util.FileExists(jsonFile) {
		return "needs_tx", "Needs Transcription"
	}
	if !util.FileExists(cutsFile) {
		return "needs_llm", "Needs Ad Detection (LLM)"
	}
	if util.FileExists(precutFile) {
		return "completed", "Completed (Ad-Free)"
	}
	data, err := os.ReadFile(cutsFile)
	var cd CutsData
	if err == nil && json.Unmarshal(data, &cd) == nil && len(cd.CutIntervals) > 0 {
		return "needs_cut", "Needs Audio Cutting"
	}
	return "completed", "Completed (0 ads)"
}

func fetchRemoteReadyCount(opts ProcOptions, config Config) (int, string) {
	if opts.Local {
		return 0, ""
	}
	reqHost := ""
	if opts.Remote {
		reqHost = config.RemoteHost
	}
	h, isRem, err := remote.ResolveProcessingHost(&config, reqHost, nil)
	if err != nil || !isRem || h == "" {
		return 0, ""
	}
	transport := remote.GetRemoteTransport()
	if !remote.IsRemoteHostReachable(h, transport) {
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
		if doneM, err := remote.LoadDoneManifest(tempDonePath); err == nil && doneM != nil {
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

func printDryRunSummary(filesCount, needsTx, needsLLM, needsCut, remotePending, alreadyComplete, remoteReady int, targetHost string, opts ProcOptions, details []dryRunFileStatus) {
	totalNeedingAction := needsTx + needsLLM + needsCut
	fmt.Println()
	fmt.Println(util.Bold("DRY RUN: Audio Processing Pipeline Status"))
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
	fmt.Printf("  Total Needing Local Processing:  %s\n", util.Bold(fmt.Sprintf("%d", totalNeedingAction)))
	if opts.Count > 0 && totalNeedingAction > opts.Count {
		fmt.Printf("  (Limit -n %d: would process first %d of %d episodes)\n", opts.Count, opts.Count, totalNeedingAction)
	}
	fmt.Println()

	if opts.Verbose {
		fmt.Println("Episode Details:")
		for _, d := range details {
			fmt.Printf("  [%s] %s\n", d.status, util.DisplayName(d.path))
		}
		fmt.Println()
	}
}

func handleProcDryRun(files []string, opts ProcOptions, config Config) {
	var needsTranscribe, needsLLM, needsCut, alreadyComplete, remotePending int
	var details []dryRunFileStatus

	for _, inputFile := range files {
		if strings.HasSuffix(inputFile, ".json") {
			continue
		}
		cat, desc := auditFileStatus(inputFile, opts)
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

	remoteReady, targetHost := fetchRemoteReadyCount(opts, config)
	printDryRunSummary(len(files), needsTranscribe, needsLLM, needsCut, remotePending, alreadyComplete, remoteReady, targetHost, opts, details)
}
