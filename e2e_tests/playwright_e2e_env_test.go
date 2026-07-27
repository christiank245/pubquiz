package e2e_tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/mxschmitt/playwright-go"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type playwrightE2EEnv struct {
	baseURL string
	dataDir string
	appCmd  *exec.Cmd
	appLogs bytes.Buffer

	pw      *playwright.Playwright
	browser playwright.Browser

	quizAdminEmail    string
	quizAdminPassword string
}

func (env *playwrightE2EEnv) setup() {
	if os.Getenv("RUN_PLAYWRIGHT_E2E") != "1" {
		Skip("set RUN_PLAYWRIGHT_E2E=1 to run Playwright browser tests")
	}

	port, err := freeLocalPort()
	Expect(err).NotTo(HaveOccurred())
	env.baseURL = fmt.Sprintf("http://127.0.0.1:%d", port)

	env.dataDir, err = os.MkdirTemp("", "pubquiz-playwright-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(func() {
		_ = os.RemoveAll(env.dataDir)
	})

	env.appCmd = exec.Command("go", "run", "..", "serve", "--http", fmt.Sprintf("127.0.0.1:%d", port), "--dir", env.dataDir, "--dev=false")
	env.appCmd.Stdout = &env.appLogs
	env.appCmd.Stderr = &env.appLogs
	Expect(env.appCmd.Start()).To(Succeed())
	Expect(waitForHTTP(env.baseURL, 45*time.Second)).To(Succeed(), env.appLogs.String())

	systemAdminEmail := fmt.Sprintf("sysadmin-%d@example.com", time.Now().UnixNano())
	systemAdminPassword := "Password123!"
	Expect(createSystemAdminAccount(env.dataDir, systemAdminEmail, systemAdminPassword)).To(Succeed(), env.appLogs.String())

	adminToken, err := loginSystemAdmin(env.baseURL, systemAdminEmail, systemAdminPassword)
	Expect(err).NotTo(HaveOccurred())

	env.quizAdminEmail = fmt.Sprintf("quizadmin-%d@example.com", time.Now().UnixNano())
	env.quizAdminPassword = "Password123!"
	Expect(createQuizAdminRecord(env.baseURL, adminToken, env.quizAdminEmail, env.quizAdminPassword)).To(Succeed())

	Expect(playwright.Install()).To(Succeed())
	env.pw, err = playwright.Run()
	Expect(err).NotTo(HaveOccurred())

	env.browser, err = env.pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	Expect(err).NotTo(HaveOccurred())
}

func (env *playwrightE2EEnv) cleanup() {
	if env.browser != nil {
		_ = env.browser.Close()
	}
	if env.pw != nil {
		_ = env.pw.Stop()
	}
	if env.appCmd != nil && env.appCmd.Process != nil {
		_ = env.appCmd.Process.Kill()
		_, _ = env.appCmd.Process.Wait()
	}
}

func freeLocalPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func waitForHTTP(baseURL string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client := &http.Client{Timeout: 2 * time.Second}
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode < http.StatusInternalServerError {
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("server not ready before timeout")
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func loginQuizAdmin(page playwright.Page, baseURL, email, password string) error {
	_, err := page.Goto(baseURL+"/quiz-admin/login", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
	if err != nil {
		return err
	}
	if err := page.Locator(`input[name="email"]`).Fill(email); err != nil {
		return err
	}
	if err := page.Locator(`input[name="password"]`).Fill(password); err != nil {
		return err
	}
	if err := page.Locator(`button:has-text("Sign in")`).Click(); err != nil {
		return err
	}
	return page.Locator(`h2:has-text("locations")`).First().WaitFor()
}

func registerNormalTeam(page playwright.Page, baseURL, teamName, email, size string) error {
	_, err := page.Goto(baseURL, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
	if err != nil {
		return err
	}
	if err := page.Locator(`a:has-text("Register team")`).First().Click(); err != nil {
		return err
	}
	if err := page.Locator(`input[name="email"]`).Fill(email); err != nil {
		return err
	}
	if err := page.Locator(`input[name="team_name"]`).Fill(teamName); err != nil {
		return err
	}
	if err := page.Locator(`input[name="team_size"]`).Fill(size); err != nil {
		return err
	}
	if err := page.Locator(`button:has-text("Register")`).Click(); err != nil {
		return err
	}
	return page.Locator(`#registration-panel >> text=Registration successful`).First().WaitFor()
}

func registerSmallTeamWithMergeConsent(page playwright.Page, baseURL, teamName, email string) error {
	_, err := page.Goto(baseURL, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
	if err != nil {
		return err
	}
	if err := page.Locator(`a:has-text("Register team")`).First().Click(); err != nil {
		return err
	}
	if err := page.Locator(`input[name="email"]`).Fill(email); err != nil {
		return err
	}
	if err := page.Locator(`input[name="team_name"]`).Fill(teamName); err != nil {
		return err
	}
	if err := page.Locator(`input[name="team_size"]`).Fill("3"); err != nil {
		return err
	}
	if err := page.Locator(`button:has-text("Register")`).Click(); err != nil {
		return err
	}
	if err := page.Locator(`text=Small teams (fewer than 4 people)`).First().WaitFor(); err != nil {
		return err
	}
	if err := page.Locator(`button:has-text("Yes, we can be merged")`).Click(); err != nil {
		return err
	}
	return page.Locator(`#registration-panel >> text=Registration successful`).First().WaitFor()
}

func fillRequiredInputInScope(page playwright.Page, scopeSelector, inputName, value string) error {
	targetSelector := fmt.Sprintf(`%s input[name="%s"]`, scopeSelector, inputName)
	target := page.Locator(targetSelector)

	count, err := target.Count()
	if err != nil {
		return fmt.Errorf("failed checking selector %q: %w", targetSelector, err)
	}
	if count == 0 {
		foundNames, namesErr := inputNamesInScope(page, scopeSelector)
		if namesErr != nil {
			return fmt.Errorf("expected input[name=%q] in %s, but it was not found", inputName, scopeSelector)
		}
		return fmt.Errorf("expected input[name=%q] in %s, but found input names: %s", inputName, scopeSelector, strings.Join(foundNames, ", "))
	}

	if err := target.First().Fill(value); err != nil {
		return fmt.Errorf("failed filling input[name=%q] in %s: %w", inputName, scopeSelector, err)
	}
	return nil
}

func inputNamesInScope(page playwright.Page, scopeSelector string) ([]string, error) {
	raw, err := page.Locator(scopeSelector).Locator("input[name]").EvaluateAll(`elements => elements.map((el) => el.getAttribute("name") || "")`)
	if err != nil {
		return nil, err
	}

	rawNames, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected evaluateAll result type: %T", raw)
	}

	names := make([]string, 0, len(rawNames))
	for _, item := range rawNames {
		name, ok := item.(string)
		if ok && strings.TrimSpace(name) != "" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return []string{"(none)"}, nil
	}
	return names, nil
}

func createSystemAdminAccount(dataDir, email, password string) error {
	cmd := exec.Command("go", "run", "..", "--dir", dataDir, "--dev=false", "admin", "create", email, password)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create system admin failed: %w (%s)", err, string(output))
	}
	return nil
}

func loginSystemAdmin(baseURL, email, password string) (string, error) {
	body, err := json.Marshal(map[string]string{
		"identity": email,
		"password": password,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/admins/auth-with-password", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("admin auth failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var payload struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.Token) == "" {
		return "", fmt.Errorf("admin auth returned empty token")
	}
	return payload.Token, nil
}

func createQuizAdminRecord(baseURL, adminToken, email, password string) error {
	username := strings.Split(email, "@")[0]
	body, err := json.Marshal(map[string]string{
		"username":        username,
		"email":           email,
		"password":        password,
		"passwordConfirm": password,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/collections/quiz_admins/records", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	responseBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("create quiz admin failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	return nil
}
