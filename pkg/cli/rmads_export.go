package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"abs/pkg/format"
	"abs/pkg/util"
)

func runExportCommand(cli CLIOptions) {
	targetArgs := cli.Args
	if len(targetArgs) == 0 {
		fmt.Println("No input files or directories specified for export.")
		return
	}
	for _, arg := range targetArgs {
		fi, err := os.Stat(arg)
		if err == nil && fi.IsDir() {
			files, _ := filepath.Glob(filepath.Join(arg, "*.transcript.json"))
			for _, f := range files {
				if cli.ExportTXT || cli.ExportFormat == "txt" {
					_, _ = format.ConvertJSONToTXT(f, nil, 0, cli.Output, cli.Quiet)
				} else {
					_, _ = format.ConvertJSONToSRT(f, nil, cli.Output, cli.Quiet)
				}
			}
		} else {
			jsonPath := arg
			if !strings.HasSuffix(jsonPath, ".json") {
				jsonPath = util.StripExt(arg) + ".transcript.json"
			}
			if cli.ExportTXT || cli.ExportFormat == "txt" {
				_, _ = format.ConvertJSONToTXT(jsonPath, nil, 0, cli.Output, cli.Quiet)
			} else {
				_, _ = format.ConvertJSONToSRT(jsonPath, nil, cli.Output, cli.Quiet)
			}
		}
	}
}
