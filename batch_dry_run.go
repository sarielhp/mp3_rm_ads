package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func handleProcDryRun(files []string, cli CLIOptions, config Config) {
	var needsTranscribe int
	var needsLLM int
	var needsCut int
	var alreadyComplete int
	var remotePending int

	type fileStatus struct {
		path   string
		status string
	}
	var details []fileStatus

	for _, inputFile := range files {
		if strings.HasSuffix(inputFile, ".json") {
			continue
		}
		mainMP3File, precutFile, _ := resolveAudioFiles(inputFile, cli)
		statFile := statusPathFor(mainMP3File)
		st, _ := loadEpisodeStatus(statFile)

		if st != nil {
			switch st.Status {
			case StateDone, StateCopiedBack, StateArchived:
				alreadyComplete++
				details = append(details, fileStatus{path: inputFile, status: "Completed (Ad-Free)"})
				continue
			case StateReadyForCopyBack:
				remotePending++
				details = append(details, fileStatus{path: inputFile, status: "Ready for Pull (Remote Done)"})
				continue
			case StateQueuedRemote, StateTranscribingRemotely, StateCuttingRemotely, StateAwaitingTranscription:
				remotePending++
				details = append(details, fileStatus{path: inputFile, status: fmt.Sprintf("Remote Processing (%s)", st.Status)})
				continue
			}
		}

		baseName := stripExt(mainMP3File)
		jsonFile := cli.TranscriptPath
		if jsonFile == "" {
			jsonFile = baseName + ".transcript.json"
		}
		cutsFile := baseName + ".cuts.json"

		hasTx := fileExists(jsonFile)
		hasCuts := fileExists(cutsFile)

		if !hasTx {
			needsTranscribe++
			details = append(details, fileStatus{path: inputFile, status: "Needs Transcription"})
		} else if !hasCuts {
			needsLLM++
			details = append(details, fileStatus{path: inputFile, status: "Needs Ad Detection (LLM)"})
		} else {
			if fileExists(precutFile) {
				alreadyComplete++
				details = append(details, fileStatus{path: inputFile, status: "Completed (Ad-Free)"})
			} else if hasCuts {
				data, err := os.ReadFile(cutsFile)
				var cd CutsData
				if err == nil && json.Unmarshal(data, &cd) == nil && len(cd.CutIntervals) > 0 {
					needsCut++
					details = append(details, fileStatus{path: inputFile, status: "Needs Audio Cutting"})
				} else {
					alreadyComplete++
					details = append(details, fileStatus{path: inputFile, status: "Completed (0 ads)"})
				}
			} else {
				alreadyComplete++
				details = append(details, fileStatus{path: inputFile, status: "Completed"})
			}
		}
	}

	totalNeedingAction := needsTranscribe + needsLLM + needsCut

	fmt.Println()
	fmt.Println(bold("DRY RUN: Audio Processing Pipeline Status"))
	fmt.Println(strings.Repeat("─", 55))
	fmt.Printf("  • Total Episodes Scanned:        %d\n", len(files))
	fmt.Printf("  • Needs Transcription (Whisper): %d\n", needsTranscribe)
	fmt.Printf("  • Needs Ad Detection (LLM):      %d\n", needsLLM)
	fmt.Printf("  • Needs Audio Cutting (FFmpeg):  %d\n", needsCut)
	if remotePending > 0 {
		fmt.Printf("  • In Remote Queue / Ready:       %d\n", remotePending)
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
