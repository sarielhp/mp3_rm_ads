package cli

import (
	"fmt"
)

func runCheckCommand(config Config, cli CLIOptions) error {
	if cli.TestKitty {
		testKittyImage(outFor(cli), cli.Args)
	} else if cli.TestGemini {
		return testGeminiAPI(outFor(cli), &config, cli.Quiet)
	} else {
		if !testWhisperServer(outFor(cli), config.WhisperURL, config.WhisperWakeCommand, cli.Quiet) {
			return fmt.Errorf("whisper test failed")
		}
	}
	return nil
}
