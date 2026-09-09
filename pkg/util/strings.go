package util

import (
	"net"
)

var netInterfaceAddrs = net.InterfaceAddrs

func ExtractHost(url string) string {
	protoEnd := -1
	for i := 0; i < len(url)-2; i++ {
		if url[i:i+3] == "://" {
			protoEnd = i + 3
			break
		}
	}
	if protoEnd < 0 {
		protoEnd = 0
	}
	hostEnd := protoEnd
	for hostEnd < len(url) && url[hostEnd] != '/' && url[hostEnd] != ':' {
		hostEnd++
	}
	return url[protoEnd:hostEnd]
}

func ExtractPort(url string) string {
	protoEnd := -1
	for i := 0; i < len(url)-2; i++ {
		if url[i:i+3] == "://" {
			protoEnd = i + 3
			break
		}
	}
	if protoEnd < 0 {
		protoEnd = 0
	}
	hostEnd := protoEnd
	for hostEnd < len(url) && url[hostEnd] != '/' && url[hostEnd] != ':' {
		hostEnd++
	}
	if hostEnd < len(url) && url[hostEnd] == ':' {
		portEnd := hostEnd + 1
		for portEnd < len(url) && url[portEnd] >= '0' && url[portEnd] <= '9' {
			portEnd++
		}
		return url[hostEnd+1 : portEnd]
	}
	return ""
}

func IsLocalHost(host string) bool {
	switch host {
	case "localhost", "127.0.0.1", "0.0.0.0", "::1":
		return true
	}
	addrs, err := netInterfaceAddrs()
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok {
			if ipnet.IP.String() == host {
				return true
			}
		}
	}
	return false
}

func SplitLines(s string) []string {
	if s == "" {
		return nil
	}
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func SplitTab(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\t' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	if start <= len(s) {
		parts = append(parts, s[start:])
	}
	return parts
}

func ToLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		b[i] = c
	}
	return string(b)
}

func ZeroWipeKey(key string) string {
	if len(key) == 0 {
		return ""
	}
	b := make([]byte, len(key))
	for i := range b {
		b[i] = '0'
	}
	return string(b)
}

func IsZeroedKey(key string) bool {
	if key == "" {
		return true
	}
	for i := 0; i < len(key); i++ {
		if key[i] != '0' {
			return false
		}
	}
	return true
}

func RepeatStr(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}

func Truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func StripExt(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			return path[:i]
		}
		if path[i] == '/' || path[i] == '\\' {
			break
		}
	}
	return path
}

func FilepathBase(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[i+1:]
		}
	}
	return path
}
