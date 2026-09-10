package adremoval

import "abs/pkg/remote"

// Adapters the engine tests share with the rest of the program.

func setRemoteTransport(t remote.RemoteTransport) {
	remote.SetRemoteTransport(t)
}

func addDoneEpisode(manifestPath string, item RemoteDoneItem) error {
	return remote.AddDoneEpisode(manifestPath, item)
}

func runRemoteAck(remoteDir string, relPaths []string) error {
	return remote.RunRemoteAck(remoteDir, relPaths)
}
