package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

const PlayerSocketPath = "/tmp/abs_player.sock"

type PlayerStatusDTO struct {
	IsRunning bool    `json:"is_running"`
	IsPaused  bool    `json:"is_paused"`
	Title     string  `json:"title"`
	Podcast   string  `json:"podcast,omitempty"`
	Path      string  `json:"path,omitempty"`
	Position  float64 `json:"position"`
	Duration  float64 `json:"duration"`
	Volume    int     `json:"volume"`
}

type mpvCommand struct {
	Command []any `json:"command"`
}

type mpvResponse struct {
	Data  any    `json:"data"`
	Error string `json:"error"`
}

type daemonState struct {
	mu       syncMutex
	path     string
	title    string
	podcast  string
	paused   bool
	start    time.Time
	offset   float64
	duration float64
}

func dialPlayerSocket() (net.Conn, error) {
	return net.DialTimeout("unix", PlayerSocketPath, 250*time.Millisecond)
}

func isPlayerSocketAlive() bool {
	conn, err := dialPlayerSocket()
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func sendPlayerIpcCommand(cmd []any) (any, error) {
	conn, err := dialPlayerSocket()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	return sendMpvRawCommand(conn, cmd)
}

func sendMpvRawCommand(conn net.Conn, cmd []any) (any, error) {
	_ = conn.SetDeadline(time.Now().Add(1 * time.Second))
	payload, err := json.Marshal(mpvCommand{Command: cmd})
	if err != nil {
		return nil, err
	}
	payload = append(payload, '\n')
	if _, err := conn.Write(payload); err != nil {
		return nil, err
	}

	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		var resp mpvResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			continue
		}
		if resp.Error != "" && resp.Error != "success" {
			return nil, fmt.Errorf("player error: %s", resp.Error)
		}
		return resp.Data, nil
	}
}

func StopPlayerSocket() error {
	if !isPlayerSocketAlive() {
		_ = os.Remove(PlayerSocketPath)
		return nil
	}
	_, err := sendPlayerIpcCommand([]any{"quit"})
	time.Sleep(100 * time.Millisecond)
	_ = os.Remove(PlayerSocketPath)
	return err
}

func PausePlayerSocket() (bool, error) {
	res, err := sendPlayerIpcCommand([]any{"cycle", "pause"})
	if err != nil {
		return false, err
	}
	if b, ok := res.(bool); ok {
		return b, nil
	}
	st, err := QueryPlayerStatus()
	if err == nil && st != nil {
		return st.IsPaused, nil
	}
	return false, nil
}

func ResumePlayerSocket() error {
	_, err := sendPlayerIpcCommand([]any{"set_property", "pause", false})
	return err
}

func SeekPlayerSocket(deltaSec float64) error {
	_, err := sendPlayerIpcCommand([]any{"seek", deltaSec, "relative"})
	return err
}

func LoadfilePlayerSocket(path string) error {
	_, err := sendPlayerIpcCommand([]any{"loadfile", path, "replace"})
	return err
}

func QueryPlayerStatus() (*PlayerStatusDTO, error) {
	conn, err := dialPlayerSocket()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	titleData, _ := sendMpvRawCommand(conn, []any{"get_property", "media-title"})
	pauseData, _ := sendMpvRawCommand(conn, []any{"get_property", "pause"})
	timeData, _ := sendMpvRawCommand(conn, []any{"get_property", "time-pos"})
	durData, _ := sendMpvRawCommand(conn, []any{"get_property", "duration"})

	title, _ := titleData.(string)
	paused, _ := pauseData.(bool)
	pos, _ := timeData.(float64)
	dur, _ := durData.(float64)

	return &PlayerStatusDTO{
		IsRunning: true,
		IsPaused:  paused,
		Title:     title,
		Position:  pos,
		Duration:  dur,
		Volume:    70,
	}, nil
}

func StartPlayerTrack(audioPath, title, podcast string) error {
	if isAudioSpawnDisabled() {
		return nil
	}

	if _, err := os.Stat(audioPath); err != nil {
		return err
	}

	if isPlayerSocketAlive() {
		if err := LoadfilePlayerSocket(audioPath); err == nil {
			_ = ResumePlayerSocket()
			return nil
		}
		_ = StopPlayerSocket()
	}

	_ = os.Remove(PlayerSocketPath)

	if _, err := exec.LookPath("mpv"); err == nil {
		return spawnDetachedMpv(audioPath, title)
	}

	return spawnDetachedDaemon(audioPath, title, podcast)
}

func spawnDetachedMpv(audioPath, title string) error {
	args := []string{
		"--no-video",
		"--no-terminal",
		fmt.Sprintf("--input-ipc-server=%s", PlayerSocketPath),
	}
	if script := findMprisScript(); script != "" {
		args = append(args, fmt.Sprintf("--script=%s", script))
	}
	if title != "" {
		args = append(args, fmt.Sprintf("--force-media-title=%s", title))
	}
	args = append(args, audioPath)

	cmd := exec.Command("mpv", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}

	for i := 0; i < 20; i++ {
		time.Sleep(25 * time.Millisecond)
		if isPlayerSocketAlive() {
			return nil
		}
	}
	return nil
}

func findMprisScript() string {
	candidates := []string{
		"/usr/lib/mpv/mpris.so",
		"/usr/lib64/mpv/mpris.so",
		"/usr/lib/x86_64-linux-gnu/mpv/mpris.so",
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(home, ".config/mpv/scripts/mpris.so"),
			filepath.Join(home, ".local/share/mpv/scripts/mpris.so"),
		)
	}
	for _, p := range candidates {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}
	return ""
}

func spawnDetachedDaemon(audioPath, title, podcast string) error {
	execPath, err := os.Executable()
	if err != nil {
		execPath = os.Args[0]
	}
	args := []string{"player", "daemon", audioPath, "--title", title, "--podcast", podcast}
	cmd := exec.Command(execPath, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}

	for i := 0; i < 20; i++ {
		time.Sleep(25 * time.Millisecond)
		if isPlayerSocketAlive() {
			return nil
		}
	}
	return nil
}

func runPlayerDaemon(audioPath, title, podcast string) error {
	_ = os.Remove(PlayerSocketPath)
	listener, err := net.Listen("unix", PlayerSocketPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = listener.Close()
		_ = os.Remove(PlayerSocketPath)
	}()

	dur := getAudioDuration(audioPath)
	state := &daemonState{
		path:     audioPath,
		title:    title,
		podcast:  podcast,
		duration: dur,
		start:    time.Now(),
	}

	cmd, err := startDaemonPlayback(audioPath)
	if err != nil {
		return err
	}

	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()

	go handleDaemonIpc(listener, state, cmd)

	<-done
	return nil
}

func startDaemonPlayback(audioPath string) (*exec.Cmd, error) {
	if _, err := exec.LookPath("cvlc"); err == nil {
		cmd := exec.Command("cvlc", "--no-video", "--intf", "dummy", "--control", "dbus", audioPath)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if err := cmd.Start(); err == nil {
			return cmd, nil
		}
	}
	if _, err := exec.LookPath("ffplay"); err == nil {
		cmd := exec.Command("ffplay", "-nodisp", "-autoexit", "-loglevel", "quiet", audioPath)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if err := cmd.Start(); err == nil {
			return cmd, nil
		}
	}
	cmd := exec.Command("mpg123", "-q", audioPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func handleDaemonIpc(listener net.Listener, state *daemonState, cmd *exec.Cmd) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		go handleDaemonConn(conn, state, cmd)
	}
}

func handleDaemonConn(conn net.Conn, state *daemonState, cmd *exec.Cmd) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var req mpvCommand
		line := scanner.Bytes()
		if err := json.Unmarshal(line, &req); err != nil || len(req.Command) == 0 {
			continue
		}
		resp := processDaemonCommand(req.Command, state, cmd)
		respBytes, _ := json.Marshal(resp)
		respBytes = append(respBytes, '\n')
		_, _ = conn.Write(respBytes)
	}
}

func processDaemonCommand(args []any, state *daemonState, cmd *exec.Cmd) mpvResponse {
	if len(args) == 0 {
		return mpvResponse{Error: "success"}
	}
	cname, _ := args[0].(string)
	switch cname {
	case "quit", "stop":
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = os.Remove(PlayerSocketPath)
		return mpvResponse{Error: "success"}
	case "cycle":
		return handleDaemonCycle(args, state, cmd)
	case "set_property":
		return handleDaemonSetProp(args, state, cmd)
	case "get_property":
		return handleDaemonGetProp(args, state)
	case "seek":
		return handleDaemonSeek(args, state)
	default:
		return mpvResponse{Error: "success"}
	}
}

func handleDaemonCycle(args []any, state *daemonState, cmd *exec.Cmd) mpvResponse {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.paused = !state.paused
	signalDaemonProcess(cmd, state.paused)
	return mpvResponse{Data: state.paused, Error: "success"}
}

func handleDaemonSetProp(args []any, state *daemonState, cmd *exec.Cmd) mpvResponse {
	if len(args) > 2 && args[1] == "pause" {
		state.mu.Lock()
		defer state.mu.Unlock()
		if p, ok := args[2].(bool); ok {
			state.paused = p
			signalDaemonProcess(cmd, state.paused)
		}
	}
	return mpvResponse{Error: "success"}
}

func signalDaemonProcess(cmd *exec.Cmd, paused bool) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	sig := syscall.SIGCONT
	if paused {
		sig = syscall.SIGSTOP
	}
	if pgid, err := syscall.Getpgid(cmd.Process.Pid); err == nil {
		_ = syscall.Kill(-pgid, sig)
	} else {
		_ = cmd.Process.Signal(sig)
	}
}

func handleDaemonGetProp(args []any, state *daemonState) mpvResponse {
	if len(args) < 2 {
		return mpvResponse{Error: "success"}
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	prop, _ := args[1].(string)
	switch prop {
	case "pause":
		return mpvResponse{Data: state.paused, Error: "success"}
	case "media-title":
		return mpvResponse{Data: state.title, Error: "success"}
	case "time-pos", "playback-time":
		pos := state.offset
		if !state.paused && !state.start.IsZero() {
			pos += time.Since(state.start).Seconds()
		}
		return mpvResponse{Data: pos, Error: "success"}
	case "duration":
		return mpvResponse{Data: state.duration, Error: "success"}
	default:
		return mpvResponse{Error: "success"}
	}
}

func handleDaemonSeek(args []any, state *daemonState) mpvResponse {
	if len(args) < 2 {
		return mpvResponse{Error: "success"}
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	delta, _ := args[1].(float64)
	if !state.paused && !state.start.IsZero() {
		state.offset += time.Since(state.start).Seconds()
		state.start = time.Now()
	}
	state.offset += delta
	if state.offset < 0 {
		state.offset = 0
	}
	return mpvResponse{Error: "success"}
}
