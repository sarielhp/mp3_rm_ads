package gmail

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// UnsubscriptionEntry stores metadata about an unsubscribe attempt.
type UnsubscriptionEntry struct {
	Timestamp   string `json:"timestamp"`
	Sender      string `json:"sender"`
	Method      string `json:"method"`
	Destination string `json:"destination"`
	Success     bool   `json:"success"`
	Error       string `json:"error,omitempty"`
}

func parseListUnsubscribe(headerVal string) (string, string) {
	var mailtoLink, httpLink string
	parts := strings.Split(headerVal, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(part, "<") || !strings.HasSuffix(part, ">") {
			continue
		}
		link := part[1 : len(part)-1]
		if strings.HasPrefix(link, "mailto:") {
			mailtoLink = link
		} else if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") {
			httpLink = link
		}
	}
	return mailtoLink, httpLink
}

func executeHTTPUnsubscribe(urlStr string) error {
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func logUnsubscription(config *Config, sender, method, destination string, unsubErr error) {
	if config.ConfigDir == "" {
		return
	}
	logPath := filepath.Join(config.ConfigDir, "unsubscriptions.json")

	var entries []UnsubscriptionEntry
	if data, err := os.ReadFile(logPath); err == nil {
		_ = json.Unmarshal(data, &entries)
	}

	errStr := ""
	if unsubErr != nil {
		errStr = unsubErr.Error()
	}

	entry := UnsubscriptionEntry{
		Timestamp:   time.Now().Format(time.RFC3339),
		Sender:      sender,
		Method:      method,
		Destination: destination,
		Success:     unsubErr == nil,
		Error:       errStr,
	}

	entries = append(entries, entry)

	if bytes, err := json.MarshalIndent(entries, "", "  "); err == nil {
		_ = os.WriteFile(logPath, bytes, 0600)
	}
}

// Exported wrappers for unsubscribe functions (used by jmap package).

func ParseListUnsubscribe(headerVal string) (string, string) {
	return parseListUnsubscribe(headerVal)
}

func ExecuteHTTPUnsubscribe(urlStr string) error {
	return executeHTTPUnsubscribe(urlStr)
}

func LogUnsubscription(config *Config, sender, method, destination string, unsubErr error) {
	logUnsubscription(config, sender, method, destination, unsubErr)
}
