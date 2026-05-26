package cmd

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gluonfield/linear-cli/api"
	"github.com/spf13/cobra"
)

const (
	defaultOAuthPort  = 8765
	defaultOAuthScope = "read,write,issues:create,comments:create"
)

type oauthConfig struct {
	ClientID     string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
	RedirectPort int    `json:"redirect_port,omitempty"`
	AccessToken  string `json:"access_token,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
	Scope        string `json:"scope,omitempty"`
	ExpiresAt    int64  `json:"expires_at,omitempty"`
}

func oauthConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "linear-cli", "auth.json"), nil
}

func loadOAuthConfig() (*oauthConfig, error) {
	path, err := oauthConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c oauthConfig
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func hasStoredOAuthToken() bool {
	c, err := loadOAuthConfig()
	if err != nil || c == nil {
		return false
	}
	return c.AccessToken != ""
}

func saveOAuthConfig(c *oauthConfig) error {
	path, err := oauthConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func mergeEnvConfig(c *oauthConfig) {
	if id := os.Getenv("LINEAR_OAUTH_CLIENT_ID"); id != "" {
		c.ClientID = id
	}
	if secret := os.Getenv("LINEAR_OAUTH_CLIENT_SECRET"); secret != "" {
		c.ClientSecret = secret
	}
	if c.RedirectPort == 0 {
		c.RedirectPort = defaultOAuthPort
	}
}

var oauthCmd = &cobra.Command{
	Use:   "oauth",
	Short: "OAuth authentication (login, logout, status, setup)",
	Long: `Authenticate with Linear via OAuth instead of a personal API key.

OAuth tokens are scoped, can be revoked per-integration, and let agent
activity be attributed to the OAuth app rather than your user. Stored at
$XDG_CONFIG_HOME/linear-cli/auth.json (mode 0600).

Setup steps (one time):
  1. Create an OAuth app: https://linear.app/settings/api/applications/new
  2. Set the redirect URL to http://localhost:8765/callback
  3. Run: linear oauth setup     (paste client id + secret)
  4. Run: linear oauth login     (opens browser)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return oauthLoginCmd.RunE(cmd, args)
	},
}

var oauthSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure OAuth client id/secret",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _ := loadOAuthConfig()
		if c == nil {
			c = &oauthConfig{}
		}
		mergeEnvConfig(c)

		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Linear OAuth client_id: ")
		id, _ := reader.ReadString('\n')
		id = strings.TrimSpace(id)
		if id != "" {
			c.ClientID = id
		}
		fmt.Print("Linear OAuth client_secret: ")
		secret, _ := reader.ReadString('\n')
		secret = strings.TrimSpace(secret)
		if secret != "" {
			c.ClientSecret = secret
		}
		fmt.Printf("Redirect port [%d]: ", c.RedirectPort)
		portStr, _ := reader.ReadString('\n')
		portStr = strings.TrimSpace(portStr)
		if portStr != "" {
			var p int
			if _, err := fmt.Sscanf(portStr, "%d", &p); err == nil && p > 0 {
				c.RedirectPort = p
			}
		}

		if c.ClientID == "" || c.ClientSecret == "" {
			return fmt.Errorf("client_id and client_secret are required")
		}
		if err := saveOAuthConfig(c); err != nil {
			return err
		}
		path, _ := oauthConfigPath()
		fmt.Printf("Saved to %s\n", path)
		fmt.Println("Next: linear oauth login")
		return nil
	},
}

var oauthLoginScope string

var oauthLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Run the OAuth browser flow and save the access token",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _ := loadOAuthConfig()
		if c == nil {
			c = &oauthConfig{}
		}
		mergeEnvConfig(c)

		if c.ClientID == "" || c.ClientSecret == "" {
			return fmt.Errorf("OAuth client not configured. Run 'linear oauth setup' or set LINEAR_OAUTH_CLIENT_ID and LINEAR_OAUTH_CLIENT_SECRET")
		}

		scope := oauthLoginScope
		if scope == "" {
			scope = defaultOAuthScope
		}

		state, err := randomState()
		if err != nil {
			return err
		}

		redirect := fmt.Sprintf("http://localhost:%d/callback", c.RedirectPort)
		authURL := buildAuthURL(c.ClientID, redirect, scope, state)

		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", c.RedirectPort))
		if err != nil {
			return fmt.Errorf("bind localhost:%d: %w (is the port free? matches your registered redirect URL?)", c.RedirectPort, err)
		}

		codeCh := make(chan string, 1)
		errCh := make(chan error, 1)
		mux := http.NewServeMux()
		mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			if e := q.Get("error"); e != "" {
				errCh <- fmt.Errorf("linear returned error: %s (%s)", e, q.Get("error_description"))
				fmt.Fprintln(w, "Authorization failed. You can close this tab.")
				return
			}
			if q.Get("state") != state {
				errCh <- fmt.Errorf("state mismatch — possible CSRF")
				fmt.Fprintln(w, "State mismatch. You can close this tab.")
				return
			}
			code := q.Get("code")
			if code == "" {
				errCh <- fmt.Errorf("no code in callback")
				return
			}
			fmt.Fprintln(w, "<h2>Authorized.</h2><p>You can close this tab and return to the terminal.</p>")
			codeCh <- code
		})

		server := &http.Server{Handler: mux}
		go server.Serve(listener)
		defer server.Shutdown(context.Background())

		fmt.Printf("Opening browser to:\n  %s\n\nIf it doesn't open, paste that URL manually.\n", authURL)
		_ = openBrowser(authURL)

		select {
		case code := <-codeCh:
			tok, err := exchangeCode(c.ClientID, c.ClientSecret, redirect, code)
			if err != nil {
				return err
			}
			c.AccessToken = tok.AccessToken
			c.TokenType = tok.TokenType
			c.Scope = tok.Scope
			if tok.ExpiresIn > 0 {
				c.ExpiresAt = time.Now().Unix() + tok.ExpiresIn
			}
			if err := saveOAuthConfig(c); err != nil {
				return err
			}
			path, _ := oauthConfigPath()
			fmt.Printf("Token saved to %s\nScopes: %s\n", path, c.Scope)
			return nil
		case err := <-errCh:
			return err
		case <-time.After(5 * time.Minute):
			return fmt.Errorf("timed out waiting for browser callback")
		}
	},
}

var oauthLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Delete the saved access token (keeps client_id/secret)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := loadOAuthConfig()
		if err != nil {
			return fmt.Errorf("no saved auth")
		}
		c.AccessToken = ""
		c.TokenType = ""
		c.Scope = ""
		c.ExpiresAt = 0
		if err := saveOAuthConfig(c); err != nil {
			return err
		}
		fmt.Println("Logged out. Run 'linear oauth login' to re-authenticate.")
		return nil
	},
}

var oauthStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show OAuth auth status",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, _ := oauthConfigPath()
		c, _ := loadOAuthConfig()
		if c == nil {
			c = &oauthConfig{}
		}
		mergeEnvConfig(c)

		fmt.Printf("Config: %s\n", path)
		fmt.Printf("Client ID:     %s\n", maskTail(c.ClientID))
		fmt.Printf("Client secret: %s\n", maskTail(c.ClientSecret))
		if c.AccessToken == "" {
			fmt.Println("Token:         (none) — run 'linear oauth login'")
			return nil
		}
		fmt.Printf("Token:         %s\n", maskTail(c.AccessToken))
		fmt.Printf("Scopes:        %s\n", c.Scope)
		if c.ExpiresAt > 0 {
			fmt.Printf("Expires:       %s\n", time.Unix(c.ExpiresAt, 0).Format(time.RFC3339))
		}

		// Verify by calling viewer
		q := `query { viewer { id name email } }`
		var res struct {
			Viewer struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				Email string `json:"email"`
			} `json:"viewer"`
		}
		if err := api.Query(q, &res); err != nil {
			fmt.Printf("Viewer check: FAILED (%v)\n", err)
			return nil
		}
		fmt.Printf("Authenticated as: %s <%s>\n", res.Viewer.Name, res.Viewer.Email)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(oauthCmd)
	oauthCmd.AddCommand(oauthSetupCmd)
	oauthCmd.AddCommand(oauthLoginCmd)
	oauthCmd.AddCommand(oauthLogoutCmd)
	oauthCmd.AddCommand(oauthStatusCmd)
	oauthLoginCmd.Flags().StringVar(&oauthLoginScope, "scope", "", "comma-separated OAuth scopes (default: read,write,issues:create,comments:create)")
}

// --- helpers ---

func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func buildAuthURL(clientID, redirect, scope, state string) string {
	u, _ := url.Parse("https://linear.app/oauth/authorize")
	q := u.Query()
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirect)
	q.Set("response_type", "code")
	q.Set("scope", scope)
	q.Set("state", state)
	q.Set("prompt", "consent")
	u.RawQuery = q.Encode()
	return u.String()
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
	Scope       string `json:"scope"`
}

func exchangeCode(clientID, clientSecret, redirect, code string) (*tokenResponse, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("redirect_uri", redirect)
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("grant_type", "authorization_code")

	req, err := http.NewRequest("POST", "https://api.linear.app/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("token exchange HTTP %d: %s", resp.StatusCode, string(body))
	}
	var tok tokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, fmt.Errorf("parse token response: %w (body: %s)", err, string(body))
	}
	if tok.AccessToken == "" {
		return nil, fmt.Errorf("no access_token in response: %s", string(body))
	}
	return &tok, nil
}

func openBrowser(u string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler"}
	default:
		cmd = "xdg-open"
	}
	args = append(args, u)
	return exec.Command(cmd, args...).Start()
}

func maskTail(s string) string {
	if s == "" {
		return "(not set)"
	}
	if len(s) <= 8 {
		return strings.Repeat("*", len(s))
	}
	return strings.Repeat("*", len(s)-6) + s[len(s)-6:]
}
