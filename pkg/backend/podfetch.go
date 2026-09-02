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

type PodFetchBackend struct {
	Host        string
	User        string
	Pass        string
	Token       string
	APIKey      string
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
	Register("podfetch", func(cfg Config) (Backend, error) {
		return NewPodFetch(cfg), nil
	})
	Register("pod_fetch", func(cfg Config) (Backend, error) {
		return NewPodFetch(cfg), nil
	})
}

func NewPodFetch(cfg Config) *PodFetchBackend {
	host := strings.TrimRight(cfg.Host, "/")
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 45 * time.Second
	}
	maxAttempts := cfg.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 10
	}

	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = cfg.Token
	}

	return &PodFetchBackend{
		Host:        host,
		User:        cfg.User,
		Pass:        cfg.Pass,
		Token:       cfg.Token,
		APIKey:      apiKey,
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

func (c *PodFetchBackend) Name() string {
	return "podfetch"
}

func (c *PodFetchBackend) getRetryDelay(attempt int) time.Duration {
	if c.RetryDelay > 0 {
		return c.RetryDelay
	}
	if attempt <= 5 {
		return 1 * time.Second
	}
	return 2 * time.Second
}

func (c *PodFetchBackend) Login() (string, error) {
	if c.APIKey != "" {
		return c.APIKey, nil
	}
	if c.Token != "" {
		return c.Token, nil
	}
	if c.User != "" && c.Pass != "" {
		return c.User, nil
	}
	if c.Host == "" && c.DBPath != "" {
		return "sqlite", nil
	}
	return "", nil
}

func (c *PodFetchBackend) TestConnection(quiet bool) (bool, error) {
	if c.Host == "" && c.DBPath == "" {
		if !quiet {
			fmt.Println("ERROR: podfetch_url or podfetch_db_path is not configured.")
		}
		return false, fmt.Errorf("podfetch_url or podfetch_db_path is not configured")
	}

	if c.Host != "" {
		if !quiet {
			fmt.Printf("Testing PodFetch server at: %s\n", c.Host)
		}
		body, err := c.Request("/api/v1/podcasts", "GET", nil)
		if err != nil {
			body, err = c.Request("/api/v1/common/health", "GET", nil)
		}
		if err != nil {
			body, err = c.Request("/", "GET", nil)
		}
		if err != nil {
			if !quiet {
				fmt.Fprintf(os.Stderr, "FAIL: Could not connect to PodFetch server: %v\n", err)
			}
			return false, err
		}
		_ = body
		if !quiet {
			fmt.Println("OK: PodFetch server is reachable.")
		}
		return true, nil
	}

	if c.DBPath != "" {
		if !quiet {
			fmt.Printf("Testing PodFetch SQLite database at: %s\n", c.DBPath)
		}
		_, err := fetchPodFetchPodcastsDB(c.DBPath)
		if err != nil {
			if !quiet {
				fmt.Fprintf(os.Stderr, "FAIL: Could not query PodFetch database: %v\n", err)
			}
			return false, err
		}
		if !quiet {
			fmt.Println("OK: PodFetch SQLite database is valid and readable.")
		}
		return true, nil
	}

	return true, nil
}

func (c *PodFetchBackend) Request(endpoint, method string, data interface{}) ([]byte, error) {
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
		req, err := buildPodFetchRequest(method, reqURL, jsonData, c.APIKey, c.Token, c.User, c.Pass)
		if err != nil {
			return nil, err
		}

		res, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxAttempts {
				time.Sleep(retryDelay)
				continue
			}
			return nil, err
		}

		body, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			lastErr = err
			if attempt < maxAttempts {
				time.Sleep(retryDelay)
				continue
			}
			return nil, err
		}

		shouldRetry, err := checkPodFetchStatus(res.StatusCode, attempt, maxAttempts, retryDelay)
		if err != nil {
			return nil, err
		}
		if shouldRetry {
			lastStatusCode = res.StatusCode
			continue
		}

		return body, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("HTTP Error %d", lastStatusCode)
}

func buildPodFetchRequest(method, reqURL string, jsonData []byte, apiKey, token, user, pass string) (*http.Request, error) {
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

	if apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
		req.Header.Set("x-api-key", apiKey)
	} else if token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	} else if user != "" && pass != "" {
		req.SetBasicAuth(user, pass)
	}
	return req, nil
}

func checkPodFetchStatus(statusCode int, attempt, maxAttempts int, retryDelay time.Duration) (bool, error) {
	if statusCode >= 500 || statusCode == 429 || statusCode == 408 {
		if attempt < maxAttempts {
			time.Sleep(retryDelay)
			return true, nil
		}
		return false, fmt.Errorf("HTTP Error %d", statusCode)
	}
	if statusCode < 200 || statusCode >= 300 {
		return false, fmt.Errorf("HTTP Error %d", statusCode)
	}
	return false, nil
}

func (c *PodFetchBackend) Libraries() ([]Library, error) {
	return []Library{
		{
			ID:        "podfetch-default",
			Name:      "PodFetch Library",
			MediaType: "podcast",
			Folders: []LibraryFolder{
				{
					ID:        "default",
					FullPath:  c.PodcastsDir,
					LibraryID: "podfetch-default",
				},
			},
		},
	}, nil
}

func (c *PodFetchBackend) PodcastLibraries() ([]Library, error) {
	return c.Libraries()
}
