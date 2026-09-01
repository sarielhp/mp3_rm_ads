package main

import (
	"github.com/sarielhp/clihelp"
)

func buildRemoteCommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "remote",
		Description: "Manage remote batch transcription and processing offloading",
		Subcommands: []clihelp.Command{
			{
				Name:        "deploy",
				Description: "Deploy current abs binary and configuration to a remote host",
				UsageLine:   "abs remote deploy [host] [options]",
				Parameters: []clihelp.Param{
					{Name: "[host]", Description: "Target remote SSH host (defaults to configured remote_host)"},
				},
				Args: clihelp.MaximumNArgs(1),
				Options: []clihelp.Option{
					clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
					clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed output"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "remote"
					opts.RemoteSubcmd = "deploy"
					if len(ctx.Args) > 0 {
						opts.RemoteHost = ctx.Args[0]
					}
					return nil
				},
			},
			{
				Name:        "push",
				Description: "Package and push audio files to remote staging for background processing",
				UsageLine:   "abs remote push <path1> [path2 ...] [options]",
				Parameters: []clihelp.Param{
					{Name: "<path1> [path2 ...]", Description: "Audio files or directories to push for remote processing"},
				},
				Options: []clihelp.Option{
					clihelp.String(&opts.RemoteHost, "--host <host>", "", "Target remote SSH host"),
					clihelp.Int(&opts.Priority, "-P, --priority <level>", 0, "Priority level for processing (higher priority processed first)"),
					clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
					clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed debug information"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "remote"
					opts.RemoteSubcmd = "push"
					opts.Args = ctx.Args
					return nil
				},
			},
			{
				Name:        "pull",
				Description: "Collect completed batch results from remote host and update local library",
				UsageLine:   "abs remote pull [host] [options]",
				Parameters: []clihelp.Param{
					{Name: "[host]", Description: "Target remote SSH host (defaults to configured remote_host)"},
				},
				Args: clihelp.MaximumNArgs(1),
				Options: []clihelp.Option{
					clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
					clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed debug information"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "remote"
					opts.RemoteSubcmd = "pull"
					if len(ctx.Args) > 0 {
						opts.RemoteHost = ctx.Args[0]
					}
					return nil
				},
			},
			{
				Name:        "scan",
				Description: "Scan remote mirror directory for pending episodes and process them",
				UsageLine:   "abs remote scan [path] [options]",
				Parameters: []clihelp.Param{
					{Name: "[path]", Description: "Target mirror directory to scan (defaults to ~/abs_remote)"},
				},
				Args: clihelp.MaximumNArgs(1),
				Options: []clihelp.Option{
					clihelp.Bool(&opts.IfDirty, "--if-dirty", false, "Only scan if .scan_trigger file exists"),
					clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
					clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed debug information"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "remote"
					opts.RemoteSubcmd = "scan"
					opts.Args = ctx.Args
					return nil
				},
			},
			{
				Name:        "start",
				Description: "Start and verify remote worker processing on a remote host",
				UsageLine:   "abs remote start [host] [options]",
				Parameters: []clihelp.Param{
					{Name: "[host]", Description: "Target remote SSH host (defaults to configured remote_host)"},
				},
				Args: clihelp.MaximumNArgs(1),
				Options: []clihelp.Option{
					clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
					clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed debug information"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "remote"
					opts.RemoteSubcmd = "start"
					opts.Args = ctx.Args
					return nil
				},
			},
			{
				Name:        "worker",
				Description: "Run remote worker scanner loop on mirror directory",
				UsageLine:   "abs remote worker [path] [options]",
				Parameters: []clihelp.Param{
					{Name: "[path]", Description: "Target mirror directory (defaults to ~/abs_remote)"},
				},
				Args: clihelp.MaximumNArgs(1),
				Options: []clihelp.Option{
					clihelp.Bool(&opts.Daemon, "-d, --daemon", false, "Run as recurring background daemon loop"),
					clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
					clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed debug information"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "remote"
					opts.RemoteSubcmd = "worker"
					opts.Args = ctx.Args
					return nil
				},
			},
			{
				Name:        "ack",
				Description: "Acknowledge pulled episodes on remote: delete audio files and archive",
				UsageLine:   "abs remote ack <path1> [path2 ...] [options]",
				Parameters: []clihelp.Param{
					{Name: "<path1> [path2 ...]", Description: "Relative paths of episodes within remote work directory"},
				},
				Args: clihelp.MinimumNArgs(1),
				Options: []clihelp.Option{
					clihelp.String(&opts.RemoteWorkDir, "--dir <path>", "", "Remote mirror root directory"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "remote"
					opts.RemoteSubcmd = "ack"
					opts.Args = ctx.Args
					return nil
				},
			},
			{
				Name:        "status",
				Description: "Check remote server status, background workers, and active batches",
				UsageLine:   "abs remote status [host] [options]",
				Parameters: []clihelp.Param{
					{Name: "[host]", Description: "Target remote SSH host (defaults to configured remote_host)"},
				},
				Args: clihelp.MaximumNArgs(1),
				Options: []clihelp.Option{
					clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
					clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed debug information"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "remote"
					opts.RemoteSubcmd = "status"
					if len(ctx.Args) > 0 {
						opts.RemoteHost = ctx.Args[0]
					}
					return nil
				},
			},
			{
				Name:        "clear",
				Description: "Stop remote workers and empty all scheduled/pending jobs from remote queue",
				UsageLine:   "abs remote clear [host] [options]",
				Parameters: []clihelp.Param{
					{Name: "[host]", Description: "Target remote SSH host (defaults to configured remote_host)"},
				},
				Args: clihelp.MaximumNArgs(1),
				Options: []clihelp.Option{
					clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "remote"
					opts.RemoteSubcmd = "clear"
					if len(ctx.Args) > 0 {
						opts.RemoteHost = ctx.Args[0]
					}
					return nil
				},
			},
			{
				Name:        "stop",
				Description: "Stop remote worker process and Whisper server on remote host",
				UsageLine:   "abs remote stop [host] [options]",
				Parameters: []clihelp.Param{
					{Name: "[host]", Description: "Target remote SSH host (defaults to configured remote_host)"},
				},
				Args: clihelp.MaximumNArgs(1),
				Options: []clihelp.Option{
					clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
					clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed debug information"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "remote"
					opts.RemoteSubcmd = "stop"
					if len(ctx.Args) > 0 {
						opts.RemoteHost = ctx.Args[0]
					}
					return nil
				},
			},
			{
				Name:        "cancel",
				Description: "Cancel a remote batch job or all active workers",
				UsageLine:   "abs remote cancel [host] [batch_id] [options]",
				Parameters: []clihelp.Param{
					{Name: "[host]", Description: "Target remote SSH host"},
					{Name: "[batch_id]", Description: "Optional batch ID to cancel"},
				},
				Args: clihelp.RangeArgs(0, 2),
				Options: []clihelp.Option{
					clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "remote"
					opts.RemoteSubcmd = "cancel"
					opts.Args = ctx.Args
					return nil
				},
			},
		},
	}
}

func buildBatchWorkerCommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "batch-worker",
		Description: "Internal background worker to process staged batch audio files",
		UsageLine:   "abs batch-worker [--batch-dir <path>] [options]",
		Options: []clihelp.Option{
			clihelp.String(&opts.BatchWorkerDir, "--batch-dir <path>", "", "Path to the staged batch directory"),
			clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
			clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed debug information"),
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "batch-worker"
			opts.Args = ctx.Args
			return nil
		},
	}
}
