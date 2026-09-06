package main

import (
	"os"
)

func handleRemoteCommand(config Config, cli CLIOptions) {
	var err error
	switch cli.RemoteSubcmd {
	case "deploy":
		err = runRemoteDeploy(&config, cli.RemoteHost, nil, cli.Quiet, cli.Verbose)
	case "push":
		err = runRemotePush(&config, cli.Args, cli.RemoteHost, nil, cli.Priority, cli.Quiet, cli.Verbose)
	case "pull":
		err = runRemotePull(&config, cli.RemoteHost, nil, cli.Quiet, cli.Verbose)
	case "clear":
		err = runRemoteClear(&config, cli.RemoteHost, nil, cli.Quiet)
	case "stop":
		err = runRemoteStop(&config, cli.RemoteHost, nil, cli.Quiet, cli.Verbose)
	case "scan", "start":
		hostCandidate := cli.RemoteHost
		if hostCandidate == "" && len(cli.Args) > 0 {
			arg := cli.Args[0]
			if fi, sErr := os.Stat(arg); sErr != nil || !fi.IsDir() {
				hostCandidate = arg
			}
		}
		targetHost, isRem, _ := ResolveProcessingHost(&config, hostCandidate, nil)
		if isRem {
			remoteWorkDir := config.RemoteWorkDir
			if remoteWorkDir == "" {
				remoteWorkDir = "~/abs_remote"
			}
			err = ensureRemoteEnvironmentAndWorker(&config, targetHost, remoteWorkDir, nil, cli.Quiet)
		} else {
			targetDir := ""
			if len(cli.Args) > 0 {
				targetDir = cli.Args[0]
			}
			err = runRemoteScan(&config, targetDir, cli.IfDirty, cli.Quiet, cli.Verbose)
		}
	case "worker":
		targetDir := ""
		if len(cli.Args) > 0 {
			targetDir = cli.Args[0]
		}
		err = runRemoteWorkerLoop(&config, targetDir, cli.Daemon, cli.Quiet, cli.Verbose)
	case "ack":
		targetDir := cli.RemoteWorkDir
		if targetDir == "" {
			targetDir = config.RemoteWorkDir
		}
		if targetDir == "" {
			targetDir = "~/abs_remote"
		}
		err = runRemoteAck(targetDir, cli.Args)
	case "status":
		err = runRemoteStatus(&config, cli.RemoteHost, nil, cli.Quiet, cli.Verbose)
	case "cancel":
		host := cli.RemoteHost
		batchID := ""
		if len(cli.Args) > 0 {
			host = cli.Args[0]
		}
		if len(cli.Args) > 1 {
			batchID = cli.Args[1]
		}
		err = runRemoteCancel(&config, host, batchID, nil, cli.Quiet)
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
