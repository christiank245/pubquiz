package main

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

var _ = Describe("playwright e2e", Ordered, func() {
	var (
		baseURL string
		dataDir string
		appCmd  *exec.Cmd
		appLogs bytes.Buffer

		pw      *playwright.Playwright
		browser playwright.Browser

		quizAdminEmail    string
		quizAdminPassword string
	)

	BeforeAll(func() {
		if os.Getenv("RUN_PLAYWRIGHT_E2E") != "1" {
			Skip("set RUN_PLAYWRIGHT_E2E=1 to run Playwright browser tests")
		}

		port, err := freeLocalPort()
		Expect(err).NotTo(HaveOccurred())
		baseURL = fmt.Sprintf("http://127.0.0.1:%d", port)

		dataDir, err = os.MkdirTemp("", "pubquiz-playwright-*")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() {
			_ = os.RemoveAll(dataDir)
		})

		appCmd = exec.Command("go", "run", ".", "serve", "--http", fmt.Sprintf("127.0.0.1:%d", port), "--dir", dataDir, "--dev=false")
		appCmd.Stdout = &appLogs
		appCmd.Stderr = &appLogs
		Expect(appCmd.Start()).To(Succeed())
		Expect(waitForHTTP(baseURL, 45*time.Second)).To(Succeed(), appLogs.String())

		systemAdminEmail := fmt.Sprintf("sysadmin-%d@example.com", time.Now().UnixNano())
		systemAdminPassword := "Password123!"
		Expect(createSystemAdminAccount(dataDir, systemAdminEmail, systemAdminPassword)).To(Succeed(), appLogs.String())

		adminToken, err := loginSystemAdmin(baseURL, systemAdminEmail, systemAdminPassword)
		Expect(err).NotTo(HaveOccurred())

		quizAdminEmail = fmt.Sprintf("quizadmin-%d@example.com", time.Now().UnixNano())
		quizAdminPassword = "Password123!"
		Expect(createQuizAdminRecord(baseURL, adminToken, quizAdminEmail, quizAdminPassword)).To(Succeed())

		Expect(playwright.Install()).To(Succeed())
		pw, err = playwright.Run()
		Expect(err).NotTo(HaveOccurred())

		browser, err = pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
			Headless: playwright.Bool(true),
		})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		if browser != nil {
			_ = browser.Close()
		}
		if pw != nil {
			_ = pw.Stop()
		}
		if appCmd != nil && appCmd.Process != nil {
			_ = appCmd.Process.Kill()
			_, _ = appCmd.Process.Wait()
		}
	})

	It("allows an end-user to register a normal team", func() {
		page, err := browser.NewPage()
		Expect(err).NotTo(HaveOccurred())
		defer page.Close()

		Expect(registerNormalTeam(page, baseURL, "Playwright Team", fmt.Sprintf("playwright-%d@example.com", time.Now().UnixNano()), "4")).To(Succeed())
	})

	It("loads the public registration panel through htmx", func() {
		page, err := browser.NewPage()
		Expect(err).NotTo(HaveOccurred())
		defer page.Close()

		_, err = page.Goto(baseURL, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())

		tile := page.Locator(`a:has-text("Register team")`).First()
		hxGet, err := tile.GetAttribute("hx-get")
		Expect(err).NotTo(HaveOccurred())
		Expect(hxGet).To(ContainSubstring("/quiz?id="))

		target, err := tile.GetAttribute("hx-target")
		Expect(err).NotTo(HaveOccurred())
		Expect(target).To(Equal("#quiz-content"))

		Expect(tile.Click()).To(Succeed())
		Expect(page.Locator(`#quiz-content #registration-panel`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`input[name="email"]`).First().WaitFor()).To(Succeed())
	})

	It("supports split-team registration for groups over 10", func() {
		page, err := browser.NewPage()
		Expect(err).NotTo(HaveOccurred())
		defer page.Close()

		_, err = page.Goto(baseURL, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(`a:has-text("Register team")`).First().Click()).To(Succeed())

		Expect(page.Locator(`input[name="email"]`).Fill(fmt.Sprintf("split-%d@example.com", time.Now().UnixNano()))).To(Succeed())
		Expect(page.Locator(`input[name="team_name"]`).Fill("Playwright Split")).To(Succeed())
		Expect(page.Locator(`input[name="team_size"]`).Fill("11")).To(Succeed())
		Expect(page.Locator(`button:has-text("Register")`).Click()).To(Succeed())

		Expect(page.Locator(`text=Teams can have at most 10 people`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`input[name="split_team_name_0"]`).Fill("Playwright Split A")).To(Succeed())
		Expect(page.Locator(`input[name="split_team_name_1"]`).Fill("Playwright Split B")).To(Succeed())
		Expect(page.Locator(`button:has-text("Yes, split and register")`).Click()).To(Succeed())
		Expect(page.Locator(`#registration-panel >> text=Registration successful`).First().WaitFor()).To(Succeed())
	})

	It("supports merge-consent prompt for small teams", func() {
		page, err := browser.NewPage()
		Expect(err).NotTo(HaveOccurred())
		defer page.Close()

		Expect(registerSmallTeamWithMergeConsent(page, baseURL, "Playwright Tiny", fmt.Sprintf("tiny-%d@example.com", time.Now().UnixNano()))).To(Succeed())
	})

	It("redirects protected admin routes to login and shows login errors", func() {
		page, err := browser.NewPage()
		Expect(err).NotTo(HaveOccurred())
		defer page.Close()

		_, err = page.Goto(baseURL+"/quiz-admin", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(`h2:has-text("Quiz Admin Login")`).First().WaitFor()).To(Succeed())

		Expect(page.Locator(`input[name="email"]`).Fill("invalid@example.com")).To(Succeed())
		Expect(page.Locator(`input[name="password"]`).Fill("wrong-password")).To(Succeed())
		Expect(page.Locator(`button:has-text("Sign in")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Invalid email or password.`).First().WaitFor()).To(Succeed())
	})

	It("wires quiz admin collection navigation through htmx targets", func() {
		page, err := browser.NewPage()
		Expect(err).NotTo(HaveOccurred())
		defer page.Close()

		Expect(loginQuizAdmin(page, baseURL, quizAdminEmail, quizAdminPassword)).To(Succeed())

		_, err = page.Goto(baseURL+"/quiz-admin/collections/locations", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())

		Expect(page.Locator(`#admin-collection-content`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`#admin-collection-sidebar`).First().WaitFor()).To(Succeed())

		addLink := page.Locator(`a:has-text("Add entry")`).First()
		hxGet, err := addLink.GetAttribute("hx-get")
		Expect(err).NotTo(HaveOccurred())
		Expect(hxGet).To(ContainSubstring("/quiz-admin/collections/locations?new=1"))

		target, err := addLink.GetAttribute("hx-target")
		Expect(err).NotTo(HaveOccurred())
		Expect(target).To(Equal("#admin-collection-content"))

		rowLink := page.Locator(`#admin-collection-content tbody tr a`).First()
		rowTarget, err := rowLink.GetAttribute("hx-target")
		Expect(err).NotTo(HaveOccurred())
		Expect(rowTarget).To(Equal("#admin-collection-content"))
	})

	It("supports quiz admin CRUD for locations and quiz dates", func() {
		page, err := browser.NewPage()
		Expect(err).NotTo(HaveOccurred())
		defer page.Close()

		Expect(loginQuizAdmin(page, baseURL, quizAdminEmail, quizAdminPassword)).To(Succeed())

		locationName := fmt.Sprintf("Playwright Pub %d", time.Now().UnixNano())
		_, err = page.Goto(baseURL+"/quiz-admin/collections/locations", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())

		Expect(page.Locator(`a:has-text("Add entry")`).Click()).To(Succeed())
		Expect(page.Locator(`#admin-edit-modal`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`#admin-edit-modal a:has-text("Close")`).Click()).To(Succeed())
		Eventually(func() bool {
			visible, err := page.Locator(`#admin-edit-modal`).First().IsVisible()
			Expect(err).NotTo(HaveOccurred())
			return visible
		}).Should(BeFalse())

		Expect(page.Locator(`a:has-text("Add entry")`).Click()).To(Succeed())
		visibleEditModal := page.Locator(`#admin-edit-modal:not(.hidden)`)
		Expect(visibleEditModal.First().WaitFor()).To(Succeed())
		Expect(visibleEditModal.Locator(`input[name="field_name"]`).Fill(locationName)).To(Succeed())
		Expect(visibleEditModal.Locator(`input[name="field_maps_url"]`).Fill("https://maps.google.com/?q=Ghent")).To(Succeed())
		Expect(visibleEditModal.Locator(`input[name="field_capacity"]`).Fill("42")).To(Succeed())
		Expect(visibleEditModal.Locator(`button:has-text("Create")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Entry created.`).First().WaitFor()).To(Succeed())
		Eventually(func() bool {
			visible, err := page.Locator(`#admin-edit-modal`).First().IsVisible()
			Expect(err).NotTo(HaveOccurred())
			return visible
		}).Should(BeFalse())
		_, err = page.Goto(baseURL+"/quiz-admin/collections/locations", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())

		locationRowLink := page.Locator(fmt.Sprintf(`tr:has-text("%s") a:has-text("%s")`, locationName, locationName)).First()
		Expect(locationRowLink.Click()).To(Succeed())
		Expect(visibleEditModal.First().WaitFor()).To(Succeed())
		Expect(visibleEditModal.Locator(`input[name="field_capacity"]`).Fill("45")).To(Succeed())
		Expect(visibleEditModal.Locator(`button:has-text("Save changes")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Entry updated.`).First().WaitFor()).To(Succeed())
		Eventually(func() bool {
			visible, err := page.Locator(`#admin-edit-modal`).First().IsVisible()
			Expect(err).NotTo(HaveOccurred())
			return visible
		}).Should(BeFalse())

		_, err = page.Goto(baseURL+"/quiz-admin/collections/quiz_dates", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(`a:has-text("Add entry")`).Click()).To(Succeed())
		Expect(visibleEditModal.First().WaitFor()).To(Succeed())
		Expect(visibleEditModal.Locator(`input[name="field_scheduled_at"]`).Fill("2027-07-08T19:45")).To(Succeed())
		_, err = visibleEditModal.Locator(`select[name="field_location"]`).SelectOption(playwright.SelectOptionValues{
			Labels: playwright.StringSlice(locationName),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(visibleEditModal.Locator(`button:has-text("Create")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Entry created.`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`text=08.07.2027 19:45`).First().WaitFor()).To(Succeed())
		_, err = page.Goto(baseURL+"/quiz-admin/collections/quiz_dates", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())

		Expect(page.Locator(`tr:has-text("08.07.2027 19:45") input[name="delete_ids"]`).First().Check()).To(Succeed())
		Expect(page.Locator(`button:has-text("Delete selected")`).Click()).To(Succeed())
		Expect(page.Locator(`#admin-delete-modal`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`#admin-delete-modal button:has-text("Delete")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Deleted 1 entrie(s).`).First().WaitFor()).To(Succeed())

		_, err = page.Goto(baseURL+"/quiz-admin/collections/locations", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s") input[name="delete_ids"]`, locationName)).First().Check()).To(Succeed())
		Expect(page.Locator(`button:has-text("Delete selected")`).Click()).To(Succeed())
		Expect(page.Locator(`#admin-delete-modal`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`#admin-delete-modal button:has-text("Delete")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Deleted 1 entrie(s).`).First().WaitFor()).To(Succeed())
	})

	It("supports registration update, merge, and delete in quiz admin", func() {
		page, err := browser.NewPage()
		Expect(err).NotTo(HaveOccurred())
		defer page.Close()

		teamA := fmt.Sprintf("Merge A %d", time.Now().UnixNano())
		teamB := fmt.Sprintf("Merge B %d", time.Now().UnixNano())
		teamEdit := fmt.Sprintf("Edit Team %d", time.Now().UnixNano())

		Expect(registerSmallTeamWithMergeConsent(page, baseURL, teamA, fmt.Sprintf("a-%d@example.com", time.Now().UnixNano()))).To(Succeed())
		Expect(registerSmallTeamWithMergeConsent(page, baseURL, teamB, fmt.Sprintf("b-%d@example.com", time.Now().UnixNano()))).To(Succeed())
		Expect(registerNormalTeam(page, baseURL, teamEdit, fmt.Sprintf("edit-%d@example.com", time.Now().UnixNano()), "4")).To(Succeed())

		Expect(loginQuizAdmin(page, baseURL, quizAdminEmail, quizAdminPassword)).To(Succeed())
		_, err = page.Goto(baseURL+"/quiz-admin/collections/registrations", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())

		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s") a:has-text("%s")`, teamEdit, teamEdit)).First().Click()).To(Succeed())
		updatedTeamName := teamEdit + " Updated"
		registrationEditModal := page.Locator(`#admin-edit-modal:not(.hidden)`)
		Expect(registrationEditModal.First().WaitFor()).To(Succeed())
		Expect(registrationEditModal.Locator(`input[name="field_team_name"]`).Fill(updatedTeamName)).To(Succeed())
		Expect(registrationEditModal.Locator(`button:has-text("Save changes")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Entry updated.`).First().WaitFor()).To(Succeed())
		Eventually(func() bool {
			visible, err := page.Locator(`#admin-edit-modal`).First().IsVisible()
			Expect(err).NotTo(HaveOccurred())
			return visible
		}).Should(BeFalse())
		_, err = page.Goto(baseURL+"/quiz-admin/collections/registrations", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())

		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s") input[name="delete_ids"]`, teamA)).First().Check()).To(Succeed())
		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s") input[name="delete_ids"]`, teamB)).First().Check()).To(Succeed())

		mergeButton := page.Locator(`#merge-entries-button`)
		Eventually(func() bool {
			disabled, checkErr := mergeButton.IsDisabled()
			Expect(checkErr).NotTo(HaveOccurred())
			return disabled
		}).Should(BeFalse())
		Expect(mergeButton.Click()).To(Succeed())
		Expect(page.Locator(`h3:has-text("Merge entries")`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`#merge-entries-modal button:has-text("Close")`).Click()).To(Succeed())
		Eventually(func() bool {
			visible, err := page.Locator(`#merge-entries-modal`).First().IsVisible()
			Expect(err).NotTo(HaveOccurred())
			return visible
		}).Should(BeFalse())
		Expect(mergeButton.Click()).To(Succeed())
		Expect(page.Locator(`h3:has-text("Merge entries")`).First().WaitFor()).To(Succeed())

		mergedName := fmt.Sprintf("Merged Team %d", time.Now().UnixNano())
		Expect(fillRequiredInputInScope(page, `#merge-entries-modal`, "team_name", mergedName)).To(Succeed())
		Expect(fillRequiredInputInScope(page, `#merge-entries-modal`, "email", fmt.Sprintf("merged-%d@example.com", time.Now().UnixNano()))).To(Succeed())
		Expect(page.Locator(`#merge-entries-modal button:has-text("Merge selected teams")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Entries merged successfully into`).First().WaitFor()).To(Succeed())
		Eventually(func() bool {
			visible, err := page.Locator(`#merge-entries-modal`).First().IsVisible()
			Expect(err).NotTo(HaveOccurred())
			return visible
		}).Should(BeFalse())
		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s")`, mergedName)).First().WaitFor()).To(Succeed())
		_, err = page.Goto(baseURL+"/quiz-admin/collections/registrations", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())

		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s") input[name="delete_ids"]`, mergedName)).First().Check()).To(Succeed())
		Expect(page.Locator(`button:has-text("Delete selected")`).Click()).To(Succeed())
		Expect(page.Locator(`#admin-delete-modal`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`#admin-delete-modal button:has-text("Delete")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Deleted 1 entrie(s).`).First().WaitFor()).To(Succeed())
	})
})

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
	cmd := exec.Command("go", "run", ".", "--dir", dataDir, "--dev=false", "admin", "create", email, password)
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
