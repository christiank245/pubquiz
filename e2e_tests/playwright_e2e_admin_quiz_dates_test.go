package e2e_tests

import (
	"fmt"
	"time"

	"github.com/mxschmitt/playwright-go"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func defineAdminQuizDateTests(env *playwrightE2EEnv) {
	It("supports quiz date CRUD and readable location labels", func() {
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		createWhen := time.Now().Add(3 * time.Hour).UTC()
		updateWhen := time.Now().Add(4 * time.Hour).UTC()
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

		locationOptions := quizDateModal.Locator(`select[name="field_location"] option:has-text("Quiz Date Label Pub")`)
		Expect(locationOptions.Count()).To(BeNumerically(">", 0))

		selectedOptions, err := quizDateModal.Locator(`select[name="field_location"]`).SelectOption(playwright.SelectOptionValues{Values: &[]string{locationID}})
		Expect(err).NotTo(HaveOccurred())
		Expect(selectedOptions).NotTo(BeEmpty())

		createLabel := createWhen.Local().Format("02.01.2006 15:04")
		Expect(quizDateModal.Locator(`input[name="field_scheduled_at"]`).Fill(createWhen.Local().Format("2006-01-02T15:04"))).To(Succeed())
		Expect(quizDateModal.Locator(`button:has-text("Create")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Entry created.`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s")`, createLabel)).First().WaitFor()).To(Succeed())
		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s")`, "Quiz Date Label Pub")).First().WaitFor()).To(Succeed())

		rowLink := page.Locator(fmt.Sprintf(`tr:has-text("%s") a:has-text("%s")`, createLabel, createLabel)).First()
		Expect(rowLink.Click()).To(Succeed())
		editModal := page.Locator(`#admin-edit-modal:not(.hidden)`)
		Expect(editModal.First().WaitFor()).To(Succeed())
		Expect(editModal.Locator(`input[name="field_scheduled_at"]`).Fill(updateWhen.Local().Format("2006-01-02T15:04"))).To(Succeed())
		Expect(editModal.Locator(`button:has-text("Save changes")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Entry updated.`).First().WaitFor()).To(Succeed())

		updatedLabel := updateWhen.Local().Format("02.01.2006 15:04")
		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s")`, updatedLabel)).First().WaitFor()).To(Succeed())

		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s") input[name="delete_ids"]`, updatedLabel)).First().Check()).To(Succeed())
		Expect(page.Locator(`button:has-text("Delete selected")`).Click()).To(Succeed())
		Expect(page.Locator(`#admin-delete-modal button:has-text("Delete")`).First().Click()).To(Succeed())
		Expect(page.Locator(`text=Deleted 1 entrie(s).`).First().WaitFor()).To(Succeed())
	})

	It("rejects invalid quiz date input", func() {
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		locationID, err := createCollectionRecord(env.baseURL, env.systemAdminToken, "locations", map[string]any{
			"name":     fmt.Sprintf("Invalid Quiz Date Pub %d", time.Now().UnixNano()),
			"maps_url": "https://maps.google.com/?q=InvalidQuizDatePub",
			"capacity": 30,
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(loginQuizAdmin(page, env.baseURL, env.quizAdminEmail, env.quizAdminPassword)).To(Succeed())
		_, err = page.Goto(env.baseURL+"/quiz-admin/collections/quiz_dates", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())

		Expect(page.Locator(`a:has-text("Add entry")`).Click()).To(Succeed())
		modal := page.Locator(`#admin-edit-modal`)
		Expect(modal.First().WaitFor()).To(Succeed())
		selectedOptions, err := modal.Locator(`select[name="field_location"]`).SelectOption(playwright.SelectOptionValues{Values: &[]string{locationID}})
		Expect(err).NotTo(HaveOccurred())
		Expect(selectedOptions).NotTo(BeEmpty())

		Expect(page.Evaluate(`() => {
			const input = document.querySelector('#admin-edit-modal input[name="field_scheduled_at"]');
			if (!input) {
				throw new Error('scheduled_at input not found');
			}
			input.setAttribute('type', 'text');
		}`)).To(Succeed())
		Expect(modal.Locator(`input[type="text"][name="field_scheduled_at"]`).Fill("not-a-date")).To(Succeed())
		Expect(page.Evaluate(`() => {
			const form = document.querySelector('#admin-edit-modal form');
			if (!form) {
				throw new Error('admin form not found');
			}
			form.noValidate = true;
			const button = form.querySelector('button[type="submit"]');
			if (!button) {
				throw new Error('admin submit button not found');
			}
			button.click();
		}`)).To(Succeed())

		Expect(page.Locator(`text=scheduled_at must be a valid date-time`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`#admin-edit-modal:not(.hidden)`).First().WaitFor()).To(Succeed())
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
