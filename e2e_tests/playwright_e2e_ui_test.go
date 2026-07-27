package e2e_tests

import (
	"github.com/mxschmitt/playwright-go"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func defineUITests(env *playwrightE2EEnv) {
	It("loads the public registration panel through htmx", func() {
		page, err := env.browser.NewPage()
		Expect(err).NotTo(HaveOccurred())
		defer page.Close()

		_, err = page.Goto(env.baseURL, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
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

	It("redirects protected admin routes to login and shows login errors", func() {
		page, err := env.browser.NewPage()
		Expect(err).NotTo(HaveOccurred())
		defer page.Close()

		_, err = page.Goto(env.baseURL+"/quiz-admin", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(`h2:has-text("Quiz Admin Login")`).First().WaitFor()).To(Succeed())

		Expect(page.Locator(`input[name="email"]`).Fill("invalid@example.com")).To(Succeed())
		Expect(page.Locator(`input[name="password"]`).Fill("wrong-password")).To(Succeed())
		Expect(page.Locator(`button:has-text("Sign in")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Invalid email or password.`).First().WaitFor()).To(Succeed())
	})

	It("wires quiz admin collection navigation through htmx targets", func() {
		page, err := env.browser.NewPage()
		Expect(err).NotTo(HaveOccurred())
		defer page.Close()

		Expect(loginQuizAdmin(page, env.baseURL, env.quizAdminEmail, env.quizAdminPassword)).To(Succeed())

		_, err = page.Goto(env.baseURL+"/quiz-admin/collections/locations", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
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
}
