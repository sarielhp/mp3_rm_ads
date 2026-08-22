package gmail

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"mail_cli/app"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gmailapi "google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

var oauthScopes = []string{
	gmailapi.GmailModifyScope,
	gmailapi.GmailSendScope,
	gmailapi.MailGoogleComScope,
	gmailapi.GmailSettingsBasicScope,
	"https://www.googleapis.com/auth/calendar.events",
}

func getOauthConfig(configDir string, scopes []string) (*oauth2.Config, error) {
	credentialsPath := filepath.Join(configDir, "credentials.json")
	if _, err := os.Stat(credentialsPath); err == nil {
		b, err := os.ReadFile(credentialsPath)
		if err != nil {
			return nil, fmt.Errorf("unable to read credentials.json: %w", err)
		}
		oauthConfig, err := google.ConfigFromJSON(b, scopes...)
		if err != nil {
			return nil, fmt.Errorf("unable to parse credentials.json: %w", err)
		}
		return oauthConfig, nil
	}

	return nil, fmt.Errorf("credentials.json not found at %s", credentialsPath)
}

// getGmailService authenticates the user and retrieves a Gmail service client.
func GetGmailService(config *Config) (*gmailapi.Service, error) {
	configDir := config.ConfigDir
	if configDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home: %w", err)
		}
		configDir = filepath.Join(homeDir, ".config", app.AppName)
	}
	tokenPath := GetTokenPath(config)

	oauthConfig, err := getOauthConfig(configDir, oauthScopes)
	if err != nil {
		return nil, err
	}

	client, err := getOAuthClient(oauthConfig, tokenPath, config)
	if err != nil {
		return nil, err
	}
	srv, err := gmailapi.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Gmail client: %w", err)
	}

	return srv, nil
}

// getOAuthClient retrieves a token, saves it, and returns the generated client.
func getOAuthClient(config *oauth2.Config, tokenPath string, appConfig *Config) (*http.Client, error) {
	tok, err := tokenFromFile(tokenPath)
	if err != nil {
		tok, err = getTokenFromWeb(config, appConfig)
		if err != nil {
			return nil, err
		}
		if err := saveToken(tokenPath, tok); err != nil {
			return nil, err
		}
	}

	// Create a token source that automatically handles refreshing
	ctx := context.Background()
	ts := config.TokenSource(ctx, tok)

	// Pre-emptively verify if the token is valid or can be successfully refreshed/re-authorized.
	// We call ts.Token() to trigger a refresh if the token is expired.
	// If refresh fails with "invalid_grant" or "token expired/revoked", we delete the token and re-auth.
	_, err = ts.Token()
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "invalid_grant") || strings.Contains(errStr, "expired") || strings.Contains(errStr, "revoked") {
			fmt.Printf("%s Cached OAuth token is invalid, expired, or revoked (%s). Re-authenticating...\n", app.PrefixWarn, errStr)
			_ = os.Remove(tokenPath)
			tok, err = getTokenFromWeb(config, appConfig)
			if err != nil {
				return nil, err
			}
			if err := saveToken(tokenPath, tok); err != nil {
				return nil, err
			}
			ts = config.TokenSource(ctx, tok)
		} else {
			return nil, fmt.Errorf("token refresh failed: %w", err)
		}
	}

	return oauth2.NewClient(ctx, ts), nil
}

// tokenFromFile retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// getTokenFromWeb requests a token from the web by starting a local server for loopback redirect.
func getTokenFromWeb(config *oauth2.Config, appConfig *Config) (*oauth2.Token, error) {
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	// Listen on a free local port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to start local listener: %w", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	config.RedirectURL = fmt.Sprintf("http://127.0.0.1:%d", port)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "Missing authorization code.")
			errChan <- fmt.Errorf("received redirect without code")
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><body style='font-family: Arial, sans-serif; text-align: center; margin-top: 50px;'>"+
			"<h1>Authentication Successful!</h1><p>You can close this window now and return to the terminal.</p></body></html>")
		codeChan <- code
	})

	srv := &http.Server{
		Handler: mux,
	}

	go func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()
	defer srv.Shutdown(context.Background())

	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce)

	fmt.Println("\n========================================================")
	fmt.Println("              GMAIL API OAUTH AUTHENTICATION            ")
	fmt.Println("========================================================")
	fmt.Println("1. Open the following URL in your browser to sign in:")
	fmt.Println()
	app.ColorBoldCyan.Println(authURL)
	fmt.Println()
	fmt.Printf("2. IF RUNNING VIA SSH: Forward port %d to your local computer first:\n", port)
	fmt.Printf("   ssh -L %d:127.0.0.1:%d user@remote-host\n\n", port, port)
	fmt.Println("3. If the redirect does not open automatically, copy the URL from your")
	fmt.Println("   browser address bar (e.g., http://127.0.0.1:.../?code=...) and paste it below.")
	fmt.Println("--------------------------------------------------------")
	fmt.Print("Paste redirected URL or authorization code here: ")

	// Fallback: spawn a goroutine to read from stdin
	// Use a context to signal the goroutine to stop when we return
	stdinCtx, stdinCancel := context.WithCancel(context.Background())
	defer stdinCancel()

	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			select {
			case <-stdinCtx.Done():
				return
			default:
			}

			input, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			input = strings.TrimSpace(input)
			if input == "" {
				continue
			}

			code := ""
			if strings.Contains(input, "code=") {
				// Handle inputs starting with domain/port like 127.0.0.1:1234/?code=... or /?code=...
				raw := input
				if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
					raw = "http://" + raw
				}
				u, err := url.Parse(raw)
				if err == nil {
					code = u.Query().Get("code")
				}
			}
			if code == "" {
				code = input
			}

			if code != "" {
				select {
				case codeChan <- code:
					return
				default:
					return
				}
			}
			fmt.Printf("%s Could not extract authorization code from input. Please try again: ", app.PrefixError)
		}
	}()

	select {
	case code := <-codeChan:
		tok, err := config.Exchange(context.Background(), code)
		if err != nil {
			return nil, fmt.Errorf("unable to exchange code for token: %w", err)
		}
		return tok, nil
	case err := <-errChan:
		return nil, fmt.Errorf("oauth error: %w", err)
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("oauth timeout after 5 minutes")
	}
}

// saveToken saves a token to a file path.
func saveToken(path string, token *oauth2.Token) error {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("unable to cache oauth token: %w", err)
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(token); err != nil {
		return fmt.Errorf("failed to encode oauth token: %w", err)
	}
	return nil
}

// OpenBrowser attempts to launch the configured or default system browser with the given URL.
// If the configured browser fails to launch, it automatically falls back to system defaults.
func OpenBrowser(browserCmd, urlStr string) error {
	if browserCmd != "" {
		cmd := exec.Command(browserCmd, urlStr)
		if err := cmd.Start(); err == nil {
			return nil
		} else {
			fmt.Printf("%s Warning: Failed to launch your configured browser %q: %v. Falling back to system defaults...\n", app.PrefixWarn, browserCmd, err)
		}
	}

	var errs []string
	switch runtime.GOOS {
	case "linux":
		launchers := []string{"xdg-open", "x-www-browser", "google-chrome", "firefox"}
		for _, launcher := range launchers {
			cmd := exec.Command(launcher, urlStr)
			if err := cmd.Start(); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", launcher, err))
			} else {
				return nil
			}
		}
		return fmt.Errorf("failed to launch any browser on linux (%s)", strings.Join(errs, ", "))
	case "darwin":
		cmd := exec.Command("open", urlStr)
		return cmd.Start()
	case "windows":
		cmd := exec.Command("cmd", "/c", "start", urlStr)
		return cmd.Start()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}
