package backend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type AudiobookshelfBackend struct {
	Host        string
	User        string
	Pass        string
	Token       string
	DBPath      string
	PodcastsDir string
	MaxAttempts int
	RetryDelay  time.Duration
	Quiet       bool
	Verbose     bool
	httpClient  *http.Client
	reqMu       syncMutex
}

func init() {
	Register("audiobookshelf", func(cfg Config) (Backend, error) {
		return NewAudiobookshelf(cfg), nil
	})
	Register("abs", func(cfg Config) (Backend, error) {
		return NewAudiobookshelf(cfg), nil
	})
}

func NewAudiobookshelf(cfg Config) *AudiobookshelfBackend {
	host := strings.TrimRight(cfg.Host, "/")
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 45 * time.Second
	}
	maxAttempts := cfg.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 10
	}

	token := cfg.Token
	if token == "" && cfg.DBPath != "" {
		token = GetTokenFromDB(cfg.DBPath)
	}

	return &AudiobookshelfBackend{
		Host:        host,
		User:        cfg.User,
		Pass:        cfg.Pass,
		Token:       token,
		DBPath:      cfg.DBPath,
		PodcastsDir: cfg.PodcastsDir,
		MaxAttempts: maxAttempts,
		RetryDelay:  cfg.RetryDelay,
		Quiet:       cfg.Quiet,
		Verbose:     cfg.Verbose,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *AudiobookshelfBackend) Name() string {
	return "audiobookshelf"
}

func (c *AudiobookshelfBackend) getRetryDelay(attempt int) time.Duration {
	if c.RetryDelay > 0 {
		return c.RetryDelay
	}
	if attempt <= 5 {
		return 1 * time.Second
	}
	return 2 * time.Second
}

func (c *AudiobookshelfBackend) Login() (string, error) {
	if c.Host == "" {
		return "", fmt.Errorf("host is not configured")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	body := fmt.Sprintf(`{"username":"%s","password":"%s"}`, c.User, c.Pass)
	req, err := http.NewRequest("POST", c.Host+"/login", strings.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return "", fmt.Errorf("failed to read login response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("login returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	var loginResp struct {
		User struct {
			Token       string `json:"token"`
			AccessToken string `json:"accessToken"`
		} `json:"user"`
	}
	if err := json.Unmarshal(data, &loginResp); err != nil {
		return "", fmt.Errorf("failed to parse login response: %w", err)
	}
	token := loginResp.User.AccessToken
	if token == "" {
		token = loginResp.User.Token
	}
	if token == "" {
		return "", fmt.Errorf("no token in login response")
	}
	c.Token = token
	return token, nil
}

func (c *AudiobookshelfBackend) TestConnection(quiet bool) (bool, error) {
	if c.Host == "" {
		if !quiet {
			fmt.Println("ERROR: audiobookshelf_url is not configured. Set it with: abs config --abs-url <url>")
		}
		return false, fmt.Errorf("audiobookshelf_url is not configured")
	}

	if !quiet {
		fmt.Printf("Testing Audiobookshelf server at: %s\n", c.Host)
	}

	if c.User == "" || c.Pass == "" {
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(c.Host)
		if err != nil {
			if !quiet {
				fmt.Fprintf(os.Stderr, "FAIL: Could not connect: %v\n", err)
			}
			return false, err
		}
		resp.Body.Close()
		if !quiet {
			fmt.Println("OK: Audiobookshelf server is reachable (no credentials configured).")
		}
		return true, nil
	}

	token, err := c.Login()
	if err != nil {
		if !quiet {
			fmt.Fprintf(os.Stderr, "FAIL: %v\n", err)
		}
		return false, err
	}

	if !quiet && token != "" {
		fmt.Println("OK: Audiobookshelf server is reachable and credentials are valid.")
	}
	return true, nil
}

func (c *AudiobookshelfBackend) Request(endpoint, method string, data interface{}) ([]byte, error) {
	if c.Host == "" {
		return nil, fmt.Errorf("host is not configured")
	}

	c.reqMu.Lock()
	defer c.reqMu.Unlock()

	reqURL := fmt.Sprintf("%s%s", c.Host, endpoint)

	var jsonData []byte
	var err error
	if data != nil {
		jsonData, err = json.Marshal(data)
		if err != nil {
			return nil, err
		}
	}

	maxAttempts := c.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 10
	}

	var lastErr error
	var lastStatusCode int

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		retryDelay := c.getRetryDelay(attempt)

		var req *http.Request
		if jsonData != nil {
			req, err = http.NewRequest(method, reqURL, bytes.NewBuffer(jsonData))
			if err == nil {
				req.Header.Set("Content-Type", "application/json")
			}
		} else {
			req, err = http.NewRequest(method, reqURL, nil)
		}
		if err != nil {
			return nil, err
		}

		if c.Token != "" {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.Token))
		}

		res, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxAttempts {
				if c.Verbose {
					fmt.Fprintf(os.Stderr, "[-] Failed to connect (attempt %d/%d). Retrying in %v...\n", attempt, maxAttempts, retryDelay)
				}
				time.Sleep(retryDelay)
				continue
			}
			if !c.Quiet {
				fmt.Fprintf(os.Stderr, "[-] Connection error to %s after %d attempts: %v\n", reqURL, maxAttempts, err)
			}
			return nil, err
		}

		body, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			lastErr = err
			if attempt < maxAttempts {
				if c.Verbose {
					fmt.Fprintf(os.Stderr, "[-] Failed reading response body (attempt %d/%d). Retrying in %v...\n", attempt, maxAttempts, retryDelay)
				}
				time.Sleep(retryDelay)
				continue
			}
			return nil, err
		}

		if res.StatusCode >= 500 || res.StatusCode == 429 || res.StatusCode == 408 {
			lastStatusCode = res.StatusCode
			if attempt < maxAttempts {
				if c.Verbose {
					fmt.Fprintf(os.Stderr, "[-] Audiobookshelf returned HTTP %d (attempt %d/%d). Retrying in %v...\n", res.StatusCode, attempt, maxAttempts, retryDelay)
				}
				time.Sleep(retryDelay)
				continue
			}
			if !c.Quiet {
				fmt.Fprintf(os.Stderr, "[-] HTTP Error %d for %s: %s\n", res.StatusCode, reqURL, string(body))
			}
			return nil, fmt.Errorf("HTTP Error %d", res.StatusCode)
		}

		if res.StatusCode < 200 || res.StatusCode >= 300 {
			if !c.Quiet {
				fmt.Fprintf(os.Stderr, "[-] HTTP Error %d for %s: %s\n", res.StatusCode, reqURL, string(body))
			}
			return nil, fmt.Errorf("HTTP Error %d", res.StatusCode)
		}

		trimmed := bytes.TrimSpace(body)
		isHTML := bytes.HasPrefix(trimmed, []byte("<")) || strings.Contains(strings.ToLower(res.Header.Get("Content-Type")), "text/html")
		if isHTML {
			if attempt < maxAttempts {
				if c.Verbose {
					fmt.Fprintf(os.Stderr, "[-] Audiobookshelf returned HTML (server waking up, attempt %d/%d). Retrying in %v...\n", attempt, maxAttempts, retryDelay)
				}
				time.Sleep(retryDelay)
				continue
			}
			if !c.Quiet {
				fmt.Fprintf(os.Stderr, "[-] Audiobookshelf returned HTML instead of JSON after %d attempts for %s: %s\n", maxAttempts, reqURL, string(body))
			}
			return nil, fmt.Errorf("server returned HTML response instead of JSON (server may still be starting up)")
		}

		return body, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("HTTP Error %d", lastStatusCode)
}
