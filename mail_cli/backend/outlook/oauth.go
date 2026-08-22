package outlook

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"mail_cli/app"
	"mail_cli/cfg_g"
)

const (
	clientID      = "14d82eec-204b-4c2f-b7e8-296a70dab67e" // Microsoft Graph Command Line Tools
	outlookScopes = "User.Read Mail.ReadWrite offline_access"
	tokenURL      = "https://login.microsoftonline.com/common/oauth2/v2.0/token"
	deviceCodeURL = "https://login.microsoftonline.com/common/oauth2/v2.0/devicecode"
)

type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
}

type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
	Message         string `json:"message"`
}

type TokenResponse struct {
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	ExpiresIn    int    `json:"expires_in"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Error        string `json:"error"`
}

func GetTokenPath(config *cfg_g.Config, accName string) string {
	configDir := config.ConfigDir
	if configDir == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			configDir = filepath.Join(homeDir, ".config", app.AppName)
		}
	}
	tokenName := fmt.Sprintf("outlook_token_%s.json", cfg_g.SanitizeLabelForCache(accName))
	return filepath.Join(configDir, tokenName)
}

func loadToken(path string) (*Token, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var tok Token
	if err := json.NewDecoder(f).Decode(&tok); err != nil {
		return nil, err
	}
	return &tok, nil
}

func saveToken(path string, tok *Token) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(tok)
}

func getNewTokenViaDeviceCode() (*Token, error) {
	fmt.Println("[*] Requesting Microsoft Device Code...")
	resp, err := http.PostForm(deviceCodeURL, url.Values{
		"client_id": {clientID},
		"scope":     {outlookScopes},
	})
	if err != nil {
		return nil, fmt.Errorf("device code request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("device code request returned status %d: %s", resp.StatusCode, string(b))
	}

	var dcr DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&dcr); err != nil {
		return nil, fmt.Errorf("failed to decode device code response: %w", err)
	}

	fmt.Println("\n==================================================================")
	fmt.Println("              OUTLOOK OAUTH DEVICE AUTHENTICATION")
	fmt.Println("==================================================================")
	fmt.Println("To sign in, please open a web browser on the page:")
	fmt.Printf("\n    ")
	app.ColorBoldCyan.Println(dcr.VerificationURI)
	fmt.Println()
	fmt.Print("And enter the following code: ")
	app.ColorBoldYellow.Print(dcr.UserCode)
	fmt.Println(" to authenticate.")
	fmt.Println("==================================================================")

	interval := time.Duration(dcr.Interval) * time.Second
	if interval == 0 {
		interval = 5 * time.Second
	}
	expiry := time.Now().Add(time.Duration(dcr.ExpiresIn) * time.Second)

	for {
		if time.Now().After(expiry) {
			return nil, fmt.Errorf("device authorization code expired")
		}
		time.Sleep(interval)

		tokenResp, err := http.PostForm(tokenURL, url.Values{
			"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
			"client_id":   {clientID},
			"device_code": {dcr.DeviceCode},
		})
		if err != nil {
			continue
		}
		defer tokenResp.Body.Close()

		var tr TokenResponse
		if err := json.NewDecoder(tokenResp.Body).Decode(&tr); err != nil {
			continue
		}

		if tr.Error != "" {
			if tr.Error == "authorization_pending" {
				continue
			}
			return nil, fmt.Errorf("oauth flow error: %s", tr.Error)
		}

		tok := &Token{
			AccessToken:  tr.AccessToken,
			RefreshToken: tr.RefreshToken,
			Expiry:       time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second),
		}
		return tok, nil
	}
}

func refreshToken(tok *Token) (*Token, error) {
	resp, err := http.PostForm(tokenURL, url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {clientID},
		"refresh_token": {tok.RefreshToken},
		"scope":         {outlookScopes},
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token refresh returned status %d: %s", resp.StatusCode, string(b))
	}

	var tr TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return nil, err
	}

	tok.AccessToken = tr.AccessToken
	if tr.RefreshToken != "" {
		tok.RefreshToken = tr.RefreshToken
	}
	tok.Expiry = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	return tok, nil
}

type TokenSource struct {
	tokenPath string
	token     *Token
}

func NewTokenSource(config *cfg_g.Config, accName string) *TokenSource {
	return &TokenSource{
		tokenPath: GetTokenPath(config, accName),
	}
}

func (ts *TokenSource) Token() (*Token, error) {
	if ts.token == nil {
		tok, err := loadToken(ts.tokenPath)
		if err != nil {
			tok, err = getNewTokenViaDeviceCode()
			if err != nil {
				return nil, err
			}
			if err := saveToken(ts.tokenPath, tok); err != nil {
				return nil, err
			}
		}
		ts.token = tok
	}

	// Buffer of 1 minute before actual expiry
	if time.Now().Add(1 * time.Minute).After(ts.token.Expiry) {
		tok, err := refreshToken(ts.token)
		if err != nil {
			// Retry full login if refresh fails
			tok, err = getNewTokenViaDeviceCode()
			if err != nil {
				return nil, err
			}
		}
		ts.token = tok
		_ = saveToken(ts.tokenPath, tok)
	}

	return ts.token, nil
}

type oauthTransport struct {
	base        http.RoundTripper
	tokenSource *TokenSource
}

func (t *oauthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	tok, err := t.tokenSource.Token()
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}

func GetOutlookClient(config *cfg_g.Config, accName string) *http.Client {
	return &http.Client{
		Transport: &oauthTransport{
			tokenSource: NewTokenSource(config, accName),
		},
		Timeout: 30 * time.Second,
	}
}
