package app

import (
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

var (
	logFile    *os.File
	tuiLogFile *os.File
	logger     *slog.Logger
	diagLogs   []string
	diagMutex  sync.Mutex
)

func AddDiagLog(msg string) {
	diagMutex.Lock()
	defer diagMutex.Unlock()
	if len(msg) > 0 && msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}
	diagLogs = append(diagLogs, msg)
	if len(diagLogs) > 300 {
		diagLogs = diagLogs[len(diagLogs)-300:]
	}
}

func GetDiagLogs() []string {
	diagMutex.Lock()
	defer diagMutex.Unlock()
	out := make([]string, len(diagLogs))
	copy(out, diagLogs)
	return out
}

type diagSplitWriter struct {
	file io.Writer
}

func (d *diagSplitWriter) Write(p []byte) (n int, err error) {
	n, err = d.file.Write(p)
	AddDiagLog(string(p))
	return n, err
}

func openLogFile(configDir, filename string) (*os.File, error) {
	logDir := filepath.Join(configDir, "logs")
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return nil, err
	}
	return os.OpenFile(filepath.Join(logDir, filename), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
}

func InitLogger(configDir string, logAppend bool) {
	if configDir == "" {
		handler := slog.NewTextHandler(os.Stderr, nil)
		logger = slog.New(handler)
		slog.SetDefault(logger)
		return
	}

	logDir := filepath.Join(configDir, "logs")

	if !logAppend {
		if files, err := os.ReadDir(logDir); err == nil {
			for _, f := range files {
				if !f.IsDir() {
					_ = os.Remove(filepath.Join(logDir, f.Name()))
				}
			}
		}
	}

	var err error
	logFile, err = openLogFile(configDir, "mail_cli.log")
	if err != nil {
		handler := slog.NewTextHandler(os.Stderr, nil)
		logger = slog.New(handler)
		slog.SetDefault(logger)
		return
	}

	log.SetOutput(logFile)

	logLevel := getLogLevelFromArgs()
	handler := slog.NewJSONHandler(logFile, &slog.HandlerOptions{
		Level: logLevel,
	})
	logger = slog.New(handler)
	slog.SetDefault(logger)
	slog.Debug("Logger initialized", slog.String("level", logLevel.String()))
}

func CloseLogger() {
	if logFile != nil {
		_ = logFile.Sync()
		_ = logFile.Close()
	}
}

func InitTuiLogger(configDir string) {
	logDir := filepath.Join(configDir, "logs")

	var err error
	tuiLogFile, err = openLogFile(configDir, "tui.log")
	if err != nil {
		if mkErr := os.MkdirAll(logDir, 0700); mkErr != nil {
			handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: getLogLevelFromArgs(),
			})
			logger = slog.New(handler)
			slog.SetDefault(logger)
			return
		}
		tuiLogFile, err = openLogFile(configDir, "tui.log")
		if err != nil {
			handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: getLogLevelFromArgs(),
			})
			logger = slog.New(handler)
			slog.SetDefault(logger)
			return
		}
	}

	splitWriter := &diagSplitWriter{file: tuiLogFile}
	log.SetOutput(splitWriter)
	logLevel := getLogLevelFromArgs()
	handler := slog.NewTextHandler(splitWriter, &slog.HandlerOptions{
		Level: logLevel,
	})
	logger = slog.New(handler)
	slog.SetDefault(logger)
	slog.Debug("TUI Logger initialized", slog.String("path", filepath.Join(logDir, "tui.log")), slog.String("level", logLevel.String()))
}

func CloseTuiLogger() {
	if tuiLogFile != nil {
		_ = tuiLogFile.Sync()
		_ = tuiLogFile.Close()
	}
}

func getLogLevelFromArgs() slog.Level {
	verboseCount := 0
	for _, arg := range os.Args {
		if arg == "-v" || arg == "--verbose" {
			verboseCount++
		} else if arg == "-vv" {
			verboseCount += 2
		} else if arg == "-vvv" {
			verboseCount += 3
		}
	}

	if verboseCount == 1 {
		return slog.LevelDebug
	} else if verboseCount > 1 {
		return slog.Level(-8)
	}
	return slog.LevelInfo
}
