package e2e_tests

import (
	"fmt"
	"time"

	"github.com/mxschmitt/playwright-go"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func defineAdminRegistrationTests(env *playwrightE2EEnv) {
	It("supports registration merge and delete flows", func() {
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		locationID, err := createCollectionRecord(env.baseURL, env.systemAdminToken, "locations", map[string]any{
			"name":     fmt.Sprintf("Merge Pub %d", time.Now().UnixNano()),
			"maps_url": "https://maps.google.com/?q=MergePub",
			"capacity": 40,
		})
		Expect(err).NotTo(HaveOccurred())
		quizID, err := createCollectionRecord(env.baseURL, env.systemAdminToken, "quiz_dates", map[string]any{
			"location":     locationID,
			"scheduled_at": time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339),
			"is_open":      true,
		})
		Expect(err).NotTo(HaveOccurred())

		teamA := fmt.Sprintf("Merge A %d", time.Now().UnixNano())
		teamB := fmt.Sprintf("Merge B %d", time.Now().UnixNano())
		teamEdit := fmt.Sprintf("Edit Team %d", time.Now().UnixNano())

		_, err = createCollectionRecord(env.baseURL, env.systemAdminToken, "registrations", map[string]any{
			"quiz":             quizID,
			"email":            fmt.Sprintf("a-%d@example.com", time.Now().UnixNano()),
			"team_name":        teamA,
			"team_size":        2,
			"willing_to_merge": true,
		})
		Expect(err).NotTo(HaveOccurred())
		_, err = createCollectionRecord(env.baseURL, env.systemAdminToken, "registrations", map[string]any{
			"quiz":             quizID,
			"email":            fmt.Sprintf("b-%d@example.com", time.Now().UnixNano()),
			"team_name":        teamB,
			"team_size":        3,
			"willing_to_merge": true,
		})
		Expect(err).NotTo(HaveOccurred())
		_, err = createCollectionRecord(env.baseURL, env.systemAdminToken, "registrations", map[string]any{
			"quiz":             quizID,
			"email":            fmt.Sprintf("edit-%d@example.com", time.Now().UnixNano()),
			"team_name":        teamEdit,
			"team_size":        4,
			"willing_to_merge": false,
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(loginQuizAdmin(page, env.baseURL, env.quizAdminEmail, env.quizAdminPassword)).To(Succeed())
		_, err = page.Goto(env.baseURL+"/quiz-admin/collections/registrations", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())

		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s") a:has-text("%s")`, teamEdit, teamEdit)).First().Click()).To(Succeed())
		Expect(page.Locator(`#admin-edit-modal:not(.hidden)`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`input[name="field_team_name"]`).Fill(teamEdit + " Updated")).To(Succeed())
		Expect(page.Locator(`button:has-text("Save changes")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Entry updated.`).First().WaitFor()).To(Succeed())

		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s") input[name="delete_ids"]`, teamA)).First().Check()).To(Succeed())
		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s") input[name="delete_ids"]`, teamB)).First().Check()).To(Succeed())
		Expect(page.Locator(`#merge-entries-button`).Click()).To(Succeed())
		Expect(page.Locator(`h3:has-text("Merge entries")`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`#merge-entries-modal button:has-text("Merge selected teams")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Entries merged successfully into`).First().WaitFor()).To(Succeed())

		Expect(page.Locator(`tr:has-text("Merged Team")`).Count()).To(Equal(0))
		_, err = page.Goto(env.baseURL+"/quiz-admin/collections/registrations", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(`button:has-text("Delete selected")`).Click()).To(Succeed())
		Expect(page.Locator(`#admin-delete-modal button:has-text("Delete")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Select at least one entry to delete.`).First().WaitFor()).To(Succeed())
	})
}
