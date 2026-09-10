package cli

import (
	"errors"
	"fmt"
	"golang.org/x/term"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

func pageText(text string) error {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		_, err := io.WriteString(os.Stdout, text)
		return err
	}
	return runTextPager(text, os.Stdout, os.Stderr)
}

func runTextPager(text string, stdout, stderr io.Writer) error {
	args := strings.Fields(os.Getenv("PAGER"))
	if len(args) > 0 {
		if _, err := exec.LookPath(args[0]); err != nil {
			args = nil
		}
	}
	if len(args) == 0 {
		for _, name := range []string{"pager", "less", "more"} {
			if path, err := exec.LookPath(name); err == nil {
				args = []string{path}
				break
			}
		}
	}
	if len(args) == 0 {
		_, err := io.WriteString(stdout, text)
		return err
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = strings.NewReader(text)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil && !errors.Is(err, syscall.EPIPE) {
		return fmt.Errorf("pager: %w", err)
	}
	return nil
}
