package cli

import "fmt"

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
	} else if cli.TestABS {
		if !testAudiobookshelfServer(config, cli.Quiet) {
			return fmt.Errorf("audiobookshelf test failed")
		}
	} else {
		if !testWhisperServer(config.WhisperURL, config.WhisperWakeCommand, cli.Quiet) {
			return fmt.Errorf("whisper test failed")
		}
	}
	return nil
}
