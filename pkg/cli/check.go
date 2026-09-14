package cli

import (
	"fmt"
)

func runCheckCommand(config Config, cli CLIOptions) error {
	if cli.TestKitty {
		testKittyImage(cli.Args)
	} else if cli.TestGemini {
		return testGeminiAPI(&config, cli.Quiet)
	} else {
		if !testWhisperServer(config.WhisperURL, config.WhisperWakeCommand, cli.Quiet) {
			return fmt.Errorf("whisper test failed")
		}
	}
	return nil
}
