package scan

import (
	"bytes"
	"log/slog"
	"mail_cli/cfg_g"
	"os/exec"
)

func TrainHamLocally(id string, emailBytes []byte, config *cfg_g.Config) {
	cmd := exec.Command("bogofilter", "-n")
	cmd.Stdin = bytes.NewReader(emailBytes)
	if err := cmd.Run(); err != nil {
		slog.Warn("Failed to run bogofilter HAM training", slog.String("message_id", id), slog.String("error", err.Error()))
	} else {
		slog.Info("Trained bogofilter that whitelisted sender address is HAM", slog.String("message_id", id))
	}
}
