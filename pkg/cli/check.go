package cli

import (
	"fmt"

	"abs/pkg/backend"
)

func runCheckCommand(config Config, cli CLIOptions) error {
	if cli.TestABSMap {
		if !absMapPodcasts(config, cli.Quiet) {
			return fmt.Errorf("abs map failed")
		}
	} else if cli.TestABSDownload {
		if !absDownloadAllData(config, cli.Quiet) {
			return fmt.Errorf("abs download failed")
		}
	} else if cli.TestKitty {
		testKittyImage(cli.Args)
	} else if cli.TestGemini {
		return testGeminiAPI(&config, cli.Quiet)
	} else if cli.TestABS {
		b, err := backend.FromAppConfig(&config, cli.Quiet)
		if err != nil {
			return fmt.Errorf("audiobookshelf test failed: %w", err)
		}
		if ok, _ := b.TestConnection(cli.Quiet); !ok {
			return fmt.Errorf("audiobookshelf test failed")
		}
	} else {
		if !testWhisperServer(config.WhisperURL, config.WhisperWakeCommand, cli.Quiet) {
			return fmt.Errorf("whisper test failed")
		}
	}
	return nil
}
