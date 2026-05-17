package main

import (
	"bufio"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"band-tui/internal/config"
	"band-tui/internal/mattermost"
)

func runAuth(cfg config.Config) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("band-tui auth")
	fmt.Println()
	fmt.Println("Band uses browser OAuth/SSO. This helper opens the login page, then saves")
	fmt.Println("the Mattermost session token from the browser cookie (MMAUTHTOKEN).")
	fmt.Println()

	method, err := askAuthMethod(reader)
	if err != nil {
		return err
	}
	authURL := buildAuthURL(cfg.ServerURL, method)
	fmt.Printf("Opening: %s\n", authURL)
	if err := openBrowser(authURL); err != nil {
		fmt.Printf("Could not open browser automatically: %v\n", err)
		fmt.Println("Open the URL above manually.")
	}

	fmt.Println()
	fmt.Println("After login, copy the cookie value:")
	fmt.Println("  DevTools → Application/Storage → Cookies → https://band.wb.ru → MMAUTHTOKEN")
	fmt.Println("You may also paste the whole Cookie header; I will extract MMAUTHTOKEN.")
	fmt.Println()
	fmt.Print("MMAUTHTOKEN or Cookie header: ")
	raw, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	token := extractMattermostToken(raw)
	if token == "" {
		return fmt.Errorf("could not find token; paste MMAUTHTOKEN value or Cookie header containing MMAUTHTOKEN=...")
	}

	checkCfg := cfg
	checkCfg.Token = token
	checkCfg.Username = ""
	checkCfg.Password = ""
	checkCfg.Mock = false

	fmt.Println("Validating token against Mattermost API…")
	client := mattermost.New(checkCfg)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	session, err := client.Connect(ctx)
	_ = client.Close()
	if err != nil {
		return fmt.Errorf("token validation failed: %w", err)
	}

	saveCfg, err := config.LoadFile(cfg.Config)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	saveCfg.ServerURL = cfg.ServerURL
	saveCfg.Token = token
	saveCfg.Username = ""
	saveCfg.Password = ""
	if cfg.Team != "" {
		saveCfg.Team = cfg.Team
	}
	if cfg.Channel != "" {
		saveCfg.Channel = cfg.Channel
	}
	if err := config.SaveFile(cfg.Config, saveCfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("Authenticated as @%s.\n", session.User.Username)
	fmt.Printf("Saved token to %s\n", cfg.Config)
	fmt.Println("Try: band-tui doctor")
	return nil
}

func askAuthMethod(reader *bufio.Reader) (string, error) {
	fmt.Println("Choose login method:")
	fmt.Println("  1) Employee Login — Wildberries Keycloak")
	fmt.Println("  2) Guest Login — wildberries.ru OAuth")
	fmt.Print("Method [1]: ")
	answer, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	switch strings.TrimSpace(strings.ToLower(answer)) {
	case "", "1", "employee", "gitlab", "keycloak":
		return "gitlab", nil
	case "2", "guest", "wb", "wbauth", "wbauth3":
		return "wb", nil
	default:
		return "", fmt.Errorf("unknown login method %q", strings.TrimSpace(answer))
	}
}

func buildAuthURL(serverURL, method string) string {
	serverURL = strings.TrimRight(serverURL, "/")
	redirect := url.QueryEscape("/")
	return fmt.Sprintf("%s/oauth/%s/login?redirect_to=%s", serverURL, method, redirect)
}

func extractMattermostToken(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "Cookie:")
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	for _, part := range strings.Split(s, ";") {
		part = strings.TrimSpace(part)
		name, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(name), "MMAUTHTOKEN") {
			return strings.Trim(strings.TrimSpace(value), "\"'")
		}
	}
	// If it was not a Cookie header, treat the input as the raw token value.
	if strings.ContainsAny(s, " \t\r\n;") || strings.Contains(s, "=") {
		return ""
	}
	return strings.Trim(s, "\"'")
}

func openBrowser(u string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	return cmd.Start()
}
