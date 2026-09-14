package cli

import (
	"os"
	"pod/pkg/remote"
)

func handleRemoteCommand(config Config, cli CLIOptions) {
	var err error
	switch cli.RemoteSubcmd {
	case "deploy":
		err = remote.RunRemoteDeploy(&config, cli.RemoteHost, nil, cli.Quiet, cli.Verbose)
	case "push":
		err = remote.RunRemotePush(&config, cli.Args, cli.RemoteHost, nil, cli.Priority, cli.Quiet, cli.Verbose)
	case "pull":
		err = remote.RunRemotePull(&config, cli.RemoteHost, nil, cli.Quiet, cli.Verbose)
	case "clear":
		err = remote.RunRemoteClear(&config, cli.RemoteHost, nil, cli.Quiet)
	case "stop":
		err = remote.RunRemoteStop(&config, cli.RemoteHost, nil, cli.Quiet, cli.Verbose)
	case "scan", "start":
		hostCandidate := cli.RemoteHost
		if hostCandidate == "" && len(cli.Args) > 0 {
			arg := cli.Args[0]
			if fi, sErr := os.Stat(arg); sErr != nil || !fi.IsDir() {
				hostCandidate = arg
			}
		}
		targetHost, isRem, _ := remote.ResolveProcessingHost(&config, hostCandidate, nil)
		if isRem {
			remoteWorkDir := config.RemoteWorkDir
			if remoteWorkDir == "" {
				remoteWorkDir = "~/abs_remote"
			}
			err = remote.EnsureRemoteEnvironmentAndWorker(&config, targetHost, remoteWorkDir, nil, cli.Quiet)
		} else {
			targetDir := ""
			if len(cli.Args) > 0 {
				targetDir = cli.Args[0]
			}
			err = remote.RunRemoteScan(&config, targetDir, cli.IfDirty, cli.Quiet, cli.Verbose)
		}
	case "worker":
		if cli.BatchWorkerDir != "" {
			handleBatchWorkerCommand(config, cli)
			return
		}
		targetDir := ""
		if len(cli.Args) > 0 {
			targetDir = cli.Args[0]
		}
		err = remote.RunRemoteWorkerLoop(&config, targetDir, cli.Daemon, cli.Quiet, cli.Verbose)
	case "ack":
		targetDir := cli.RemoteWorkDir
		if targetDir == "" {
			targetDir = config.RemoteWorkDir
		}
		if targetDir == "" {
			targetDir = "~/abs_remote"
		}
		err = remote.RunRemoteAck(targetDir, cli.Args)
	case "status":
		err = remote.RunRemoteStatus(&config, cli.RemoteHost, nil, cli.Quiet, cli.Verbose)
	case "cancel":
		host := cli.RemoteHost
		batchID := ""
		if len(cli.Args) > 0 {
			host = cli.Args[0]
		}
		if len(cli.Args) > 1 {
			batchID = cli.Args[1]
		}
		err = remote.RunRemoteCancel(&config, host, batchID, nil, cli.Quiet)
	default:
		return
	}

	if err != nil {
		fatalError("Remote error: %v\n", err)
	}
}

func handleBatchWorkerCommand(config Config, cli CLIOptions) {
	batchDir := cli.BatchWorkerDir
	if batchDir == "" && len(cli.Args) > 0 {
		batchDir = cli.Args[0]
	}
	if err := runBatchWorker(batchDir, cli.Quiet, cli.Verbose); err != nil {
		fatalError("Worker error: %v\n", err)
	}
}
