package e2e_tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mxschmitt/playwright-go"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type playwrightE2EEnv struct {
	baseURL     string
	dataDir     string
	videoDir    string
	recordVideo bool
	appCmd      *exec.Cmd
	appLogs     bytes.Buffer

	pw      *playwright.Playwright
	browser playwright.Browser

	quizAdminEmail    string
	quizAdminPassword string
	systemAdminToken  string
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

	env.recordVideo = strings.TrimSpace(os.Getenv("PLAYWRIGHT_E2E_RECORD_VIDEO")) == "1"
	if env.recordVideo {
		cwd, err := os.Getwd()
		Expect(err).NotTo(HaveOccurred())
		env.videoDir = filepath.Join(cwd, ".playwright-videos", time.Now().Format("20060102-150405"))
		Expect(os.MkdirAll(env.videoDir, 0o755)).To(Succeed())
		GinkgoWriter.Printf("Playwright videos will be written to %s\n", env.videoDir)
	}

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
	env.systemAdminToken = adminToken

	env.quizAdminEmail = fmt.Sprintf("quizadmin-%d@example.com", time.Now().UnixNano())
	env.quizAdminPassword = "Password123!"
	username := strings.Split(env.quizAdminEmail, "@")[0]
	body, err := json.Marshal(map[string]string{
		"username":        username,
		"email":           env.quizAdminEmail,
		"password":        env.quizAdminPassword,
		"passwordConfirm": env.quizAdminPassword,
	})
	Expect(err).NotTo(HaveOccurred())

	req, err := http.NewRequest(http.MethodPost, env.baseURL+"/api/collections/quiz_admins/records", bytes.NewReader(body))
	Expect(err).NotTo(HaveOccurred())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := http.DefaultClient.Do(req)
	Expect(err).NotTo(HaveOccurred())
	defer resp.Body.Close()
	responseBody, _ := io.ReadAll(resp.Body)
	Expect(resp.StatusCode).To(BeNumerically("<", http.StatusBadRequest), strings.TrimSpace(string(responseBody)))

	Expect(playwright.Install()).To(Succeed())
	env.pw, err = playwright.Run()
	Expect(err).NotTo(HaveOccurred())

	launchOptions := playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	}
	if slowMoMS := strings.TrimSpace(os.Getenv("PLAYWRIGHT_E2E_SLOWMO_MS")); slowMoMS != "" {
		ms, parseErr := strconv.Atoi(slowMoMS)
		Expect(parseErr).NotTo(HaveOccurred())
		Expect(ms).To(BeNumerically(">", 0))
		launchOptions.SlowMo = playwright.Float(float64(ms))
	}

	env.browser, err = env.pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: launchOptions.Headless,
		SlowMo:   launchOptions.SlowMo,
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

func (env *playwrightE2EEnv) newPage() (playwright.Page, func() error, error) {
	specName := sanitizeVideoName(CurrentSpecReport().FullText())
	var context playwright.BrowserContext
	var err error
	if env.recordVideo {
		context, err = env.browser.NewContext(playwright.BrowserNewContextOptions{
			RecordVideo: &playwright.RecordVideo{
				Dir: playwright.String(env.videoDir),
				Size: &playwright.Size{
					Width:  1280,
					Height: 720,
				},
				ShowActions: &playwright.ShowAction{
					Duration: playwright.Float(1000),
					Position: playwright.AnnotatePositionTopRight,
					FontSize: playwright.Int(20),
				},
			},
		})
	} else {
		context, err = env.browser.NewContext()
	}
	if err != nil {
		return nil, nil, err
	}

	page, err := context.NewPage()
	if err != nil {
		_ = context.Close()
		return nil, nil, err
	}

	cleanup := func() error {
		if err := page.Close(); err != nil {
			_ = context.Close()
			return err
		}
		if err := context.Close(); err != nil {
			return err
		}
		if env.recordVideo {
			return renameRecordedVideo(env.videoDir, specName)
		}
		return nil
	}

	return page, cleanup, nil
}

func renameRecordedVideo(recordDir, specName string) error {
	entries, err := os.ReadDir(recordDir)
	if err != nil {
		return err
	}
	var videoPath string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(strings.ToLower(name), ".webm") {
			videoPath = filepath.Join(recordDir, name)
			break
		}
	}
	if videoPath == "" {
		return fmt.Errorf("no video file found in %s", recordDir)
	}

	targetPath := filepath.Join(recordDir, specName+".webm")
	if err := os.RemoveAll(targetPath); err != nil {
		return err
	}
	if videoPath == targetPath {
		return nil
	}
	return os.Rename(videoPath, targetPath)
}

func sanitizeVideoName(value string) string {
	replacer := regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
	cleaned := strings.TrimSpace(value)
	cleaned = replacer.ReplaceAllString(cleaned, "-")
	cleaned = strings.Trim(cleaned, "-_.")
	if cleaned == "" {
		return "spec"
	}
	return cleaned
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

type collectionRecordsResponse struct {
	Items []map[string]any `json:"items"`
}

func createCollectionRecord(baseURL, adminToken, collection string, payload any) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	endpoint := fmt.Sprintf("%s/api/collections/%s/records", strings.TrimRight(baseURL, "/"), url.PathEscape(collection))
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

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
		return "", fmt.Errorf("create %s record failed with status %d: %s", collection, resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(responseBody, &created); err != nil {
		return "", err
	}
	if strings.TrimSpace(created.ID) == "" {
		return "", fmt.Errorf("create %s record returned empty id", collection)
	}
	return created.ID, nil
}

func listCollectionRecords(baseURL, adminToken, collection string) ([]map[string]any, error) {
	endpoint := fmt.Sprintf("%s/api/collections/%s/records", strings.TrimRight(baseURL, "/"), url.PathEscape(collection))
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("list %s records failed with status %d: %s", collection, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload collectionRecordsResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	return payload.Items, nil
}

func updateCollectionRecord(baseURL, adminToken, collection, recordID string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("%s/api/collections/%s/records/%s", strings.TrimRight(baseURL, "/"), url.PathEscape(collection), url.PathEscape(recordID))
	req, err := http.NewRequest(http.MethodPatch, endpoint, bytes.NewReader(body))
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

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("update %s record %s failed with status %d: %s", collection, recordID, resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	return nil
}

func deleteCollectionRecord(baseURL, adminToken, collection, recordID string) error {
	endpoint := fmt.Sprintf("%s/api/collections/%s/records/%s", strings.TrimRight(baseURL, "/"), url.PathEscape(collection), url.PathEscape(recordID))
	req, err := http.NewRequest(http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("delete %s record %s failed with status %d: %s", collection, recordID, resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	return nil
}

func deleteCollection(baseURL, adminToken, collection string) error {
	endpoint := fmt.Sprintf("%s/api/collections/%s", strings.TrimRight(baseURL, "/"), url.PathEscape(collection))
	req, err := http.NewRequest(http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("delete collection %s failed with status %d: %s", collection, resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	return nil
}
