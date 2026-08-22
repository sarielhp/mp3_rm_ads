package tui

import (
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"

	"mail_cli/app"
	"mail_cli/cfg_g"

	tea "github.com/charmbracelet/bubbletea"
)

func InitTuiMode(config interface{}, labelPrefix string, bk *Backend) error {
	app.TuiActive = true
	cfg := config.(*cfg_g.Config)
	cfg.Quiet = true
	if err := bk.RunPreFlightCheck(cfg); err != nil {
		return err
	}
	client, err := bk.GetClientForSelected(cfg)
	if err != nil {
		return err
	}
	if err := client.Validate(); err != nil {
		return err
	}
	bk.InitTuiLogger(cfg.ConfigDir)
	defer bk.CloseTuiLogger()

	defer func() {
		if r := recover(); r != nil {
			slog.Error("TUI panicked", slog.Any("panic", r), slog.String("stack", string(debug.Stack())))
			fmt.Fprintf(os.Stderr, "\n%s TUI Panicked: %v\nSee logs/tui.log for full details.\n", app.PrefixError, r)
		}
	}()

	model := NewTuiModel(client, labelPrefix, bk)
	model.cfg = cfg
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
