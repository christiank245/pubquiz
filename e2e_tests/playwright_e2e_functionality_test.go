package e2e_tests

import (
	"fmt"
	"time"

	"github.com/mxschmitt/playwright-go"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func defineFunctionalityTests(env *playwrightE2EEnv) {
	It("allows an end-user to register a normal team", func() {
		page, err := env.browser.NewPage()
		Expect(err).NotTo(HaveOccurred())
		defer page.Close()

		Expect(registerNormalTeam(page, env.baseURL, "Playwright Team", fmt.Sprintf("playwright-%d@example.com", time.Now().UnixNano()), "4")).To(Succeed())
	})

	It("supports split-team registration for groups over 10", func() {
		page, err := env.browser.NewPage()
		Expect(err).NotTo(HaveOccurred())
		defer page.Close()

		_, err = page.Goto(env.baseURL, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
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
		page, err := env.browser.NewPage()
		Expect(err).NotTo(HaveOccurred())
		defer page.Close()

		Expect(registerSmallTeamWithMergeConsent(page, env.baseURL, "Playwright Tiny", fmt.Sprintf("tiny-%d@example.com", time.Now().UnixNano()))).To(Succeed())
	})

	It("supports quiz admin CRUD for locations and quiz dates", func() {
		page, err := env.browser.NewPage()
		Expect(err).NotTo(HaveOccurred())
		defer page.Close()

		Expect(loginQuizAdmin(page, env.baseURL, env.quizAdminEmail, env.quizAdminPassword)).To(Succeed())

		locationName := fmt.Sprintf("Playwright Pub %d", time.Now().UnixNano())
		_, err = page.Goto(env.baseURL+"/quiz-admin/collections/locations", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
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
		_, err = page.Goto(env.baseURL+"/quiz-admin/collections/locations", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
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

		_, err = page.Goto(env.baseURL+"/quiz-admin/collections/quiz_dates", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
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
		_, err = page.Goto(env.baseURL+"/quiz-admin/collections/quiz_dates", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())

		Expect(page.Locator(`tr:has-text("08.07.2027 19:45") input[name="delete_ids"]`).First().Check()).To(Succeed())
		Expect(page.Locator(`button:has-text("Delete selected")`).Click()).To(Succeed())
		Expect(page.Locator(`#admin-delete-modal`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`#admin-delete-modal button:has-text("Delete")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Deleted 1 entrie(s).`).First().WaitFor()).To(Succeed())

		_, err = page.Goto(env.baseURL+"/quiz-admin/collections/locations", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s") input[name="delete_ids"]`, locationName)).First().Check()).To(Succeed())
		Expect(page.Locator(`button:has-text("Delete selected")`).Click()).To(Succeed())
		Expect(page.Locator(`#admin-delete-modal`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`#admin-delete-modal button:has-text("Delete")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Deleted 1 entrie(s).`).First().WaitFor()).To(Succeed())
	})

	It("supports registration update, merge, and delete in quiz admin", func() {
		page, err := env.browser.NewPage()
		Expect(err).NotTo(HaveOccurred())
		defer page.Close()

		teamA := fmt.Sprintf("Merge A %d", time.Now().UnixNano())
		teamB := fmt.Sprintf("Merge B %d", time.Now().UnixNano())
		teamEdit := fmt.Sprintf("Edit Team %d", time.Now().UnixNano())

		Expect(registerSmallTeamWithMergeConsent(page, env.baseURL, teamA, fmt.Sprintf("a-%d@example.com", time.Now().UnixNano()))).To(Succeed())
		Expect(registerSmallTeamWithMergeConsent(page, env.baseURL, teamB, fmt.Sprintf("b-%d@example.com", time.Now().UnixNano()))).To(Succeed())
		Expect(registerNormalTeam(page, env.baseURL, teamEdit, fmt.Sprintf("edit-%d@example.com", time.Now().UnixNano()), "4")).To(Succeed())

		Expect(loginQuizAdmin(page, env.baseURL, env.quizAdminEmail, env.quizAdminPassword)).To(Succeed())
		_, err = page.Goto(env.baseURL+"/quiz-admin/collections/registrations", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
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
		_, err = page.Goto(env.baseURL+"/quiz-admin/collections/registrations", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
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
		_, err = page.Goto(env.baseURL+"/quiz-admin/collections/registrations", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())

		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s") input[name="delete_ids"]`, mergedName)).First().Check()).To(Succeed())
		Expect(page.Locator(`button:has-text("Delete selected")`).Click()).To(Succeed())
		Expect(page.Locator(`#admin-delete-modal`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`#admin-delete-modal button:has-text("Delete")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Deleted 1 entrie(s).`).First().WaitFor()).To(Succeed())
	})
}
