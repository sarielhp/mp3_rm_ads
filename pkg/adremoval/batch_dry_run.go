package adremoval

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"pod/pkg/pipeline"
	"pod/pkg/types"
	"pod/pkg/util"
)

type dryRunFileStatus struct {
	path   string
	status string
}

func auditFileStatus(inputFile string, opts types.ProcOptions) (category string, statusText string) {
	mainMP3File, precutFile, _ := pipeline.ResolveAudioFiles(inputFile, opts.Verbose)
	statFile := pipeline.StatusPathFor(mainMP3File)
	st, _ := pipeline.LoadEpisodeStatus(statFile)

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
	var cd types.CutsData
	if err == nil && json.Unmarshal(data, &cd) == nil && len(cd.CutIntervals) > 0 {
		return "needs_cut", "Needs Audio Cutting"
	}
	return "completed", "Completed (0 ads)"
}

func printDryRunSummary(filesCount, needsTx, needsLLM, needsCut, remotePending, alreadyComplete int, opts types.ProcOptions, details []dryRunFileStatus) {
	totalNeedingAction := needsTx + needsLLM + needsCut
	fmt.Println()
	fmt.Println(util.Bold("DRY RUN: Audio Processing Pipeline Status"))
	fmt.Println(strings.Repeat("─", 55))
	fmt.Printf("  • Total Episodes Scanned:        %d\n", filesCount)
	fmt.Printf("  • Needs Transcription (Whisper): %d\n", needsTx)
	fmt.Printf("  • Needs Ad Detection (LLM):      %d\n", needsLLM)
	fmt.Printf("  • Needs Audio Cutting (FFmpeg):  %d\n", needsCut)
	fmt.Printf("  • Already Processed / Ad-Free:   %d\n", alreadyComplete)
	if remotePending > 0 {
		// Left mid-flight by the remote processing this build no longer has.
		// Reported so they stay visible rather than silently uncounted.
		fmt.Printf("  • Stranded in a remote state:    %d\n", remotePending)
	}
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

func handleProcDryRun(files []string, opts types.ProcOptions, cfg types.Config) {
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

	printDryRunSummary(len(files), needsTranscribe, needsLLM, needsCut, remotePending, alreadyComplete, opts, details)
}
