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
	Short: "OAuth authentication (login, logout, status)",
	Long: `Authenticate with Linear via OAuth instead of a personal API key.

OAuth tokens are scoped, can be revoked per-integration, and let agent
activity be attributed to the OAuth app rather than your user. Stored at
$XDG_CONFIG_HOME/linear-cli/auth.json (mode 0600).

First-time use:
  1. Create an OAuth app: https://linear.app/settings/api/applications/new
     - Redirect URL: http://localhost:8765/callback
  2. Run: linear oauth login
     (it prompts for client_id/secret on first run, then opens the browser)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return oauthLoginCmd.RunE(cmd, args)
	},
}

var oauthLoginScope string

var oauthLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Linear (prompts for client credentials on first run)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _ := loadOAuthConfig()
		if c == nil {
			c = &oauthConfig{}
		}
		mergeEnvConfig(c)

		if c.ClientID == "" || c.ClientSecret == "" {
			fmt.Println("First-time setup. Create an OAuth app at:")
			fmt.Println("  https://linear.app/settings/api/applications/new")
			fmt.Printf("Redirect URL must be: http://localhost:%d/callback\n\n", c.RedirectPort)
			reader := bufio.NewReader(os.Stdin)
			if c.ClientID == "" {
				fmt.Print("client_id: ")
				v, _ := reader.ReadString('\n')
				c.ClientID = strings.TrimSpace(v)
			}
			if c.ClientSecret == "" {
				fmt.Print("client_secret: ")
				v, _ := reader.ReadString('\n')
				c.ClientSecret = strings.TrimSpace(v)
			}
			if c.ClientID == "" || c.ClientSecret == "" {
				return fmt.Errorf("client_id and client_secret are required")
			}
			if err := saveOAuthConfig(c); err != nil {
				return err
			}
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

		codeCh := make(chan string, 1)
		errCh := make(chan error, 1)

		// Try to bind the local callback listener. On headless / remote
		// machines this may fail or be unreachable from the user's browser;
		// we fall back to manual paste below either way.
		listener, listenErr := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", c.RedirectPort))
		var server *http.Server
		if listenErr == nil {
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
			server = &http.Server{Handler: mux}
			go server.Serve(listener)
			defer server.Shutdown(context.Background())
		}

		fmt.Printf("Open this URL in your browser to authorize:\n\n  %s\n\n", authURL)
		if listenErr != nil {
			fmt.Printf("(could not bind localhost:%d locally: %v — manual paste only)\n\n", c.RedirectPort, listenErr)
		} else {
			_ = openBrowser(authURL)
			fmt.Println("If you're on this machine, the browser will redirect back automatically.")
		}
		fmt.Println("If you're on a remote/headless machine, after approving:")
		fmt.Println("  - your browser will land on a localhost URL that won't load — that's expected")
		fmt.Println("  - copy the entire URL from the browser address bar and paste it here.")
		fmt.Print("\nPaste callback URL (or press Enter to keep waiting): ")

		// Read manual paste from stdin in a goroutine so we can race it
		// against the local listener.
		go func() {
			reader := bufio.NewReader(os.Stdin)
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimSpace(line)
			if line == "" {
				return
			}
			code, perr := parseCodeFromCallback(line, state)
			if perr != nil {
				errCh <- perr
				return
			}
			codeCh <- code
		}()

		var code string
		select {
		case code = <-codeCh:
		case err := <-errCh:
			return err
		case <-time.After(10 * time.Minute):
			return fmt.Errorf("timed out waiting for authorization")
		}

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
		fmt.Printf("\nToken saved to %s\nScopes: %s\n", path, c.Scope)
		return nil
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

func parseCodeFromCallback(input, expectedState string) (string, error) {
	// Accept either a full URL ("http://localhost:8765/callback?code=...&state=...")
	// or just the query string ("code=...&state=...") or just the code.
	var q url.Values
	if strings.Contains(input, "?") {
		u, err := url.Parse(input)
		if err != nil {
			return "", fmt.Errorf("parse pasted URL: %w", err)
		}
		q = u.Query()
	} else if strings.Contains(input, "=") {
		v, err := url.ParseQuery(input)
		if err != nil {
			return "", fmt.Errorf("parse pasted query string: %w", err)
		}
		q = v
	} else {
		// Bare code — no state to verify. Warn but accept.
		fmt.Fprintln(os.Stderr, "warning: bare code pasted; state not verified")
		return input, nil
	}
	if e := q.Get("error"); e != "" {
		return "", fmt.Errorf("linear returned error: %s (%s)", e, q.Get("error_description"))
	}
	if got := q.Get("state"); got != "" && got != expectedState {
		return "", fmt.Errorf("state mismatch — pasted URL is not from this login attempt")
	}
	code := q.Get("code")
	if code == "" {
		return "", fmt.Errorf("no 'code' parameter in pasted URL")
	}
	return code, nil
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
