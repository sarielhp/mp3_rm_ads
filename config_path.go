package main

import (
	"net"
	"os"
	"path/filepath"
	"strings"
)

func userTmpDir() string {
	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("LOGNAME")
	}
	if username == "" {
		username = "user"
	}
	dir := filepath.Join(os.TempDir(), username, "abs")
	_ = os.MkdirAll(dir, 0755)
	return dir
}

func configDir() string {
	if testConfigPath != "" {
		return filepath.Dir(testConfigPath)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return userTmpDir()
	}
	return filepath.Join(home, configDirName)
}

func configPath() string {
	if testConfigPath != "" {
		return testConfigPath
	}
	return filepath.Join(configDir(), configFileName)
}

func legacyConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, legacyConfigDirName, configFileName)
}

func opencodeConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, opencodeConfigFile)
}

func localIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			ip := ipnet.IP.String()
			if !ipnet.IP.IsMulticast() && !ipnet.IP.IsLinkLocalUnicast() {
				return ip
			}
		}
	}
	return "127.0.0.1"
}

func replaceIP(url, ip string) string {
	return strings.Replace(url, "192.168.1.230", ip, 1)
}
