package e2e_tests

import (
	"github.com/mxschmitt/playwright-go"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func defineAdminGeneralTests(env *playwrightE2EEnv) {
	It("redirects protected admin routes to login and shows login errors", func() {
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		_, err = page.Goto(env.baseURL+"/quiz-admin", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(`h2:has-text("Quiz Admin Login")`).First().WaitFor()).To(Succeed())

		Expect(page.Locator(`input[name="email"]`).Fill("invalid@example.com")).To(Succeed())
		Expect(page.Locator(`input[name="password"]`).Fill("wrong-password")).To(Succeed())
		Expect(page.Locator(`button:has-text("Sign in")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Invalid email or password.`).First().WaitFor()).To(Succeed())
	})

	It("logs out quiz admins and protects admin routes", func() {
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		Expect(loginQuizAdmin(page, env.baseURL, env.quizAdminEmail, env.quizAdminPassword)).To(Succeed())
		Expect(page.URL()).To(ContainSubstring("/quiz-admin/collections/locations"))
		Expect(page.Locator(`header details > summary`).Click()).To(Succeed())
		Expect(page.Locator(`a:has-text("Dashboard")`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`a:has-text("Public website")`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`button:has-text("Logout")`).Click()).To(Succeed())
		Expect(page.Locator(`h2:has-text("Quiz Admin Login")`).First().WaitFor()).To(Succeed())
	})

	It("wires quiz admin collection navigation through htmx targets", func() {
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		Expect(loginQuizAdmin(page, env.baseURL, env.quizAdminEmail, env.quizAdminPassword)).To(Succeed())
		_, err = page.Goto(env.baseURL+"/quiz-admin/collections/locations", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())

		Expect(page.Locator(`#admin-collection-content`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`#admin-collection-sidebar`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`#admin-collection-sidebar a:has-text("locations")`).Count()).To(Equal(1))
		Expect(page.Locator(`#admin-collection-sidebar a:has-text("quiz_dates")`).Count()).To(Equal(1))
		Expect(page.Locator(`#admin-collection-sidebar a:has-text("registrations")`).Count()).To(Equal(1))
		Expect(page.Locator(`#admin-collection-sidebar a:has-text("quiz_admins")`).Count()).To(Equal(0))

		Expect(page.Locator(`#admin-collection-sidebar a:has-text("quiz_dates")`).Click()).To(Succeed())
		Expect(page.Locator(`#admin-collection-content`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`#admin-collection-content h2:has-text("quiz_dates")`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`#admin-collection-sidebar`).First().WaitFor()).To(Succeed())

		Expect(page.Locator(`#admin-collection-sidebar a:has-text("registrations")`).Click()).To(Succeed())
		Expect(page.Locator(`#admin-collection-content`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`#admin-collection-content h2:has-text("registrations")`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`#admin-collection-sidebar`).First().WaitFor()).To(Succeed())

		Expect(page.Locator(`header details > summary`).Click()).To(Succeed())
		Expect(page.Locator(`button:has-text("Logout")`).First().WaitFor()).To(Succeed())
	})
}
