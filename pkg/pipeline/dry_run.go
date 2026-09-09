package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sariel/abs/pkg/types"
	"github.com/sariel/abs/pkg/util"
)

type DryRunFileStatus struct {
	Path   string
	Status string
}

func AuditFileStatus(inputFile string, cli types.CLIOptions) (category string, statusText string) {
	mainMP3File, precutFile, _ := ResolveAudioFiles(inputFile, cli.Verbose)
	statFile := StatusPathFor(mainMP3File)
	st, _ := LoadEpisodeStatus(statFile)

	if st != nil {
		switch st.Status {
		case types.StateDone, types.StateCopiedBack, types.StateArchived:
			return "completed", "Completed (Ad-Free)"
		case types.StateReadyForCopyBack:
			return "remote_pending", "Ready for Pull (Remote Done)"
		case types.StateQueuedRemote, types.StateTranscribingRemotely, types.StateCuttingRemotely, types.StateAwaitingTranscription:
			return "remote_pending", fmt.Sprintf("Remote Processing (%s)", st.Status)
		}
	}

	baseName := util.StripExt(mainMP3File)
	jsonFile := cli.TranscriptPath
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
	var cd types.CutsData
	if err == nil && json.Unmarshal(data, &cd) == nil && len(cd.CutIntervals) > 0 {
		return "needs_cut", "Needs Audio Cutting"
	}
	return "completed", "Completed (0 ads)"
}

func PrintDryRunSummary(filesCount, needsTx, needsLLM, needsCut, remotePending, alreadyComplete, remoteReady int, targetHost string, cli types.CLIOptions, details []DryRunFileStatus) {
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
	if cli.Count > 0 && totalNeedingAction > cli.Count {
		fmt.Printf("  (Limit -n %d: would process first %d of %d episodes)\n", cli.Count, cli.Count, totalNeedingAction)
	}
	fmt.Println()

	if cli.Verbose {
		fmt.Println("Episode Details:")
		for _, d := range details {
			fmt.Printf("  [%s] %s\n", d.Status, filepath.Base(d.Path))
		}
		fmt.Println()
	}
}

func HandleProcDryRun(files []string, cli types.CLIOptions, cfg types.Config) {
	var needsTranscribe, needsLLM, needsCut, alreadyComplete, remotePending int
	var details []DryRunFileStatus

	for _, inputFile := range files {
		if strings.HasSuffix(inputFile, ".json") {
			continue
		}
		cat, desc := AuditFileStatus(inputFile, cli)
		details = append(details, DryRunFileStatus{Path: inputFile, Status: desc})
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

	PrintDryRunSummary(len(files), needsTranscribe, needsLLM, needsCut, remotePending, alreadyComplete, 0, "", cli, details)
}
