package main

import (
	"fmt"
)

var dockerMu syncMutex

func fetchDockerLogs(containerName string, tail int) string {
	if containerName == "" {
		return ""
	}
	dockerMu.Lock()
	defer dockerMu.Unlock()

	cmd := execCommand("docker", "logs", "--tail", fmt.Sprintf("%d", tail), containerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return string(output)
}
