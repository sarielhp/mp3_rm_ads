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

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		retryDelay := c.getRetryDelay(attempt)
		body, shouldRetry, err := c.executeRequestAttempt(method, reqURL, jsonData, attempt, maxAttempts, retryDelay)
		if err != nil {
			return nil, err
		}
		if shouldRetry {
			continue
		}
		return body, nil
	}

	return nil, fmt.Errorf("request failed after %d attempts", maxAttempts)
}

func (c *AudiobookshelfBackend) executeRequestAttempt(method, reqURL string, jsonData []byte, attempt, maxAttempts int, retryDelay time.Duration) ([]byte, bool, error) {
	req, err := buildHTTPRequest(method, reqURL, jsonData, c.Token)
	if err != nil {
		return nil, false, err
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		if attempt < maxAttempts {
			if c.Verbose {
				fmt.Fprintf(os.Stderr, "[-] Failed to connect (attempt %d/%d). Retrying in %v...\n", attempt, maxAttempts, retryDelay)
			}
			time.Sleep(retryDelay)
			return nil, true, nil
		}
		if !c.Quiet {
			fmt.Fprintf(os.Stderr, "[-] Connection error to %s after %d attempts: %v\n", reqURL, maxAttempts, err)
		}
		return nil, false, err
	}

	body, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		if attempt < maxAttempts {
			if c.Verbose {
				fmt.Fprintf(os.Stderr, "[-] Failed reading response body (attempt %d/%d). Retrying in %v...\n", attempt, maxAttempts, retryDelay)
			}
			time.Sleep(retryDelay)
			return nil, true, nil
		}
		return nil, false, err
	}

	shouldRetry, err := c.checkResponseStatus(res.StatusCode, body, reqURL, attempt, maxAttempts, retryDelay)
	if err != nil || shouldRetry {
		return nil, shouldRetry, err
	}

	shouldRetryHTML, err := c.checkHTMLResponse(res, body, reqURL, attempt, maxAttempts, retryDelay)
	if err != nil || shouldRetryHTML {
		return nil, shouldRetryHTML, err
	}

	return body, false, nil
}

func buildHTTPRequest(method, reqURL string, jsonData []byte, token string) (*http.Request, error) {
	var req *http.Request
	var err error
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
	if token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}
	return req, nil
}

func (c *AudiobookshelfBackend) checkResponseStatus(statusCode int, body []byte, reqURL string, attempt, maxAttempts int, retryDelay time.Duration) (bool, error) {
	if statusCode == http.StatusUnauthorized {
		if attempt < maxAttempts && c.User != "" && c.Pass != "" {
			c.Login()
			time.Sleep(retryDelay)
			return true, nil
		}
		return false, fmt.Errorf("HTTP Error 401 Unauthorized")
	}

	if statusCode >= 500 || statusCode == 429 || statusCode == 408 {
		if attempt < maxAttempts {
			if c.Verbose {
				fmt.Fprintf(os.Stderr, "[-] Audiobookshelf returned HTTP %d (attempt %d/%d). Retrying in %v...\n", statusCode, attempt, maxAttempts, retryDelay)
			}
			time.Sleep(retryDelay)
			return true, nil
		}
		if !c.Quiet {
			fmt.Fprintf(os.Stderr, "[-] HTTP Error %d for %s: %s\n", statusCode, reqURL, string(body))
		}
		return false, fmt.Errorf("HTTP Error %d", statusCode)
	}

	if statusCode < 200 || statusCode >= 300 {
		if !c.Quiet {
			fmt.Fprintf(os.Stderr, "[-] HTTP Error %d for %s: %s\n", statusCode, reqURL, string(body))
		}
		return false, fmt.Errorf("HTTP Error %d", statusCode)
	}
	return false, nil
}

func (c *AudiobookshelfBackend) checkHTMLResponse(res *http.Response, body []byte, reqURL string, attempt, maxAttempts int, retryDelay time.Duration) (bool, error) {
	trimmed := bytes.TrimSpace(body)
	isHTML := bytes.HasPrefix(trimmed, []byte("<")) || strings.Contains(strings.ToLower(res.Header.Get("Content-Type")), "text/html")
	if !isHTML {
		return false, nil
	}
	if attempt < maxAttempts {
		if c.Verbose {
			fmt.Fprintf(os.Stderr, "[-] Audiobookshelf returned HTML (server waking up, attempt %d/%d). Retrying in %v...\n", attempt, maxAttempts, retryDelay)
		}
		time.Sleep(retryDelay)
		return true, nil
	}
	if !c.Quiet {
		fmt.Fprintf(os.Stderr, "[-] Audiobookshelf returned HTML instead of JSON after %d attempts for %s: %s\n", maxAttempts, reqURL, string(body))
	}
	return false, fmt.Errorf("server returned HTML response instead of JSON (server may still be starting up)")
}
