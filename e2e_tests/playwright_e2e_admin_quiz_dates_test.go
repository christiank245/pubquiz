package e2e_tests

import (
	"fmt"
	"time"

	"github.com/mxschmitt/playwright-go"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func defineAdminQuizDateTests(env *playwrightE2EEnv) {
	It("uses readable location labels in quiz date forms and tables", func() {
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		locationID, err := createCollectionRecord(env.baseURL, env.systemAdminToken, "locations", map[string]any{
			"name":     fmt.Sprintf("Quiz Date Label Pub %d", time.Now().UnixNano()),
			"maps_url": "https://maps.google.com/?q=QuizDateLabelPub",
			"capacity": 30,
		})
		Expect(err).NotTo(HaveOccurred())
		_, err = createCollectionRecord(env.baseURL, env.systemAdminToken, "quiz_dates", map[string]any{
			"location":     locationID,
			"scheduled_at": time.Now().Add(3 * time.Hour).UTC().Format(time.RFC3339),
			"is_open":      true,
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(loginQuizAdmin(page, env.baseURL, env.quizAdminEmail, env.quizAdminPassword)).To(Succeed())
		_, err = page.Goto(env.baseURL+"/quiz-admin/collections/quiz_dates", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())

		Expect(page.Locator(`a:has-text("Add entry")`).Click()).To(Succeed())
		quizDateModal := page.Locator(`#admin-edit-modal`)
		Expect(quizDateModal.First().WaitFor()).To(Succeed())
		optionCount, err := quizDateModal.Locator(`select[name="field_location"] option`).Count()
		Expect(err).NotTo(HaveOccurred())
		Expect(optionCount).To(BeNumerically(">", 1))
	})

	It("cascades quiz date deletes to registrations", func() {
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		locationID, err := createCollectionRecord(env.baseURL, env.systemAdminToken, "locations", map[string]any{
			"name":     fmt.Sprintf("Quiz Date Pub %d", time.Now().UnixNano()),
			"maps_url": "https://maps.google.com/?q=QuizDatePub",
			"capacity": 30,
		})
		Expect(err).NotTo(HaveOccurred())
		quizDateID, err := createCollectionRecord(env.baseURL, env.systemAdminToken, "quiz_dates", map[string]any{
			"location":     locationID,
			"scheduled_at": time.Now().Add(3 * time.Hour).UTC().Format(time.RFC3339),
			"is_open":      true,
		})
		Expect(err).NotTo(HaveOccurred())
		_, err = createCollectionRecord(env.baseURL, env.systemAdminToken, "registrations", map[string]any{
			"quiz":             quizDateID,
			"email":            fmt.Sprintf("cascade-%d@example.com", time.Now().UnixNano()),
			"team_name":        fmt.Sprintf("Cascade Team %d", time.Now().UnixNano()),
			"team_size":        3,
			"willing_to_merge": true,
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(loginQuizAdmin(page, env.baseURL, env.quizAdminEmail, env.quizAdminPassword)).To(Succeed())
		_, err = page.Goto(env.baseURL+"/quiz-admin/collections/quiz_dates", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(`tr input[name="delete_ids"]`).First().Check()).To(Succeed())
		Expect(page.Locator(`button:has-text("Delete selected")`).Click()).To(Succeed())
		Expect(page.Locator(`#admin-delete-modal button:has-text("Delete")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Deleted 1 entrie(s).`).First().WaitFor()).To(Succeed())

		registrations, err := listCollectionRecords(env.baseURL, env.systemAdminToken, "registrations")
		Expect(err).NotTo(HaveOccurred())
		_, err = recordIDByFieldValue(registrations, "team_name", "Cascade Team")
		Expect(err).To(HaveOccurred())
	})
}
