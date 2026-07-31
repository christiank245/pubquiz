package e2e_tests

import (
	"fmt"
	"time"

	"github.com/mxschmitt/playwright-go"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func defineAdminLocationTests(env *playwrightE2EEnv) {
	It("supports CRUD for locations", func() {
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		Expect(loginQuizAdmin(page, env.baseURL, env.quizAdminEmail, env.quizAdminPassword)).To(Succeed())

		locationName := fmt.Sprintf("Playwright Pub %d", time.Now().UnixNano())
		_, err = page.Goto(env.baseURL+"/quiz-admin/collections/locations", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())

		Expect(page.Locator(`a:has-text("Add entry")`).Click()).To(Succeed())
		Expect(page.Locator(`#admin-edit-modal`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`#admin-edit-modal a:has-text("Close")`).Click()).To(Succeed())

		Expect(page.Locator(`a:has-text("Add entry")`).Click()).To(Succeed())
		visibleEditModal := page.Locator(`#admin-edit-modal:not(.hidden)`)
		Expect(visibleEditModal.First().WaitFor()).To(Succeed())
		Expect(visibleEditModal.Locator(`input[name="field_name"]`).Fill(locationName)).To(Succeed())
		Expect(visibleEditModal.Locator(`input[name="field_maps_url"]`).Fill("https://maps.google.com/?q=Ghent")).To(Succeed())
		Expect(visibleEditModal.Locator(`input[name="field_capacity"]`).Fill("42")).To(Succeed())
		Expect(visibleEditModal.Locator(`button:has-text("Create")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Entry created.`).First().WaitFor()).To(Succeed())

		locationRowLink := page.Locator(fmt.Sprintf(`tr:has-text("%s") a:has-text("%s")`, locationName, locationName)).First()
		Expect(locationRowLink.Click()).To(Succeed())
		visibleEditModal = page.Locator(`#admin-edit-modal:not(.hidden)`)
		Expect(visibleEditModal.First().WaitFor()).To(Succeed())
		Expect(visibleEditModal.Locator(`input[name="field_capacity"]`).Fill("45")).To(Succeed())
		Expect(visibleEditModal.Locator(`button:has-text("Save changes")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Entry updated.`).First().WaitFor()).To(Succeed())

		_, err = page.Goto(env.baseURL+"/quiz-admin/collections/locations", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s") input[name="delete_ids"]`, locationName)).First().Check()).To(Succeed())
		Expect(page.Locator(`button:has-text("Delete selected")`).Click()).To(Succeed())
		Expect(page.Locator(`#admin-delete-modal button:has-text("Delete")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Deleted 1 entrie(s).`).First().WaitFor()).To(Succeed())
	})

	It("rejects invalid location input", func() {
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		Expect(loginQuizAdmin(page, env.baseURL, env.quizAdminEmail, env.quizAdminPassword)).To(Succeed())
		_, err = page.Goto(env.baseURL+"/quiz-admin/collections/locations", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())

		Expect(page.Locator(`a:has-text("Add entry")`).Click()).To(Succeed())
		modal := page.Locator(`#admin-edit-modal`)
		Expect(modal.First().WaitFor()).To(Succeed())
		Expect(modal.Locator(`input[name="field_name"]`).Fill(fmt.Sprintf("Invalid Location %d", time.Now().UnixNano()))).To(Succeed())
		Expect(modal.Locator(`input[name="field_maps_url"]`).Fill("https://maps.google.com/?q=InvalidLocation")).To(Succeed())
		Expect(page.Evaluate(`() => {
			const input = document.querySelector('#admin-edit-modal input[name="field_capacity"]');
			if (!input) {
				throw new Error('capacity input not found');
			}
			input.setAttribute('type', 'text');
		}`)).To(Succeed())
		Expect(modal.Locator(`input[type="text"][name="field_capacity"]`).Fill("abc")).To(Succeed())
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

		Expect(page.Locator(`text=capacity must be a valid number`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`#admin-edit-modal:not(.hidden)`).First().WaitFor()).To(Succeed())
	})
}
