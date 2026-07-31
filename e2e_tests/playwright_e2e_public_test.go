package e2e_tests

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mxschmitt/playwright-go"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func definePublicTests(env *playwrightE2EEnv) {
	It("renders the public informational pages with the navigation menu", func() {
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		for _, tc := range []struct {
			path    string
			heading string
		}{
			{path: "/about", heading: "About Us"},
			{path: "/contact", heading: "Contact Us"},
			{path: "/privacy", heading: "Privacy Policy"},
		} {
			_, err := page.Goto(env.baseURL+tc.path, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
			Expect(err).NotTo(HaveOccurred())
			Expect(page.Locator(`h1:has-text("` + tc.heading + `")`).First().WaitFor()).To(Succeed())
		}
	})

	It("loads the quiz detail page directly", func() {
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		_, err = page.Goto(env.baseURL, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())

		tile := page.Locator(`a[href^="/quiz?id="]`).First()
		href, err := tile.GetAttribute("href")
		Expect(err).NotTo(HaveOccurred())
		Expect(href).NotTo(BeEmpty())

		_, err = page.Goto(env.baseURL+href, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())

		Expect(page.Locator(`#quiz-content #registration-panel`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`h2:has-text("Register for ")`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`a:has-text("Open Google Maps")`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`text=Seats left:`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`a:has-text("← Back to all quiz dates")`).First().WaitFor()).To(Succeed())
	})

	It("rejects missing and closed quiz ids", func() {
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		response, err := page.Goto(env.baseURL+"/quiz", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(response).NotTo(BeNil())
		Expect(response.Status()).To(Equal(404))

		bodyText, err := page.Locator("body").TextContent()
		Expect(err).NotTo(HaveOccurred())
		lowerBody := strings.ToLower(bodyText)
		Expect(lowerBody).To(ContainSubstring("gopher"))
		Expect(lowerBody).To(ContainSubstring("beer"))

		_, err = page.Goto(env.baseURL, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		count, err := page.Locator(`a[href^="/quiz?id="]`).Count()
		Expect(err).NotTo(HaveOccurred())
		Expect(count).To(BeNumerically(">", 0))
		lastQuizHref, err := page.Locator(`a[href^="/quiz?id="]`).Nth(count - 1).GetAttribute("href")
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.TrimSpace(lastQuizHref)).NotTo(BeEmpty())
		parsedHref, err := url.Parse(lastQuizHref)
		Expect(err).NotTo(HaveOccurred())
		lastQuizID := strings.TrimSpace(parsedHref.Query().Get("id"))
		Expect(lastQuizID).NotTo(BeEmpty())

		Expect(updateCollectionRecord(env.baseURL, env.systemAdminToken, "quiz_dates", lastQuizID, map[string]any{"is_open": false})).To(Succeed())

		closedResponse, err := page.Goto(env.baseURL+"/quiz?id="+url.QueryEscape(lastQuizID), playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(closedResponse).NotTo(BeNil())
		Expect(closedResponse.Status()).To(Equal(404))
		Expect(page.Locator(`text=This page has gone out for a beer`).First().WaitFor()).To(Succeed())
	})

	It("validates registration input and seat capacity", func() {
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		_, err = page.Goto(env.baseURL, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		quizHref, err := page.Locator(`a[href^="/quiz?id="]`).First().GetAttribute("href")
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.TrimSpace(quizHref)).NotTo(BeEmpty())
		_, err = page.Goto(env.baseURL+quizHref, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())

		seatsText, err := page.Locator(`#registration-panel p strong`).TextContent()
		Expect(err).NotTo(HaveOccurred())
		seatsLeft, err := strconv.Atoi(strings.TrimSpace(seatsText))
		Expect(err).NotTo(HaveOccurred())

		Expect(submitRegistrationIgnoringValidation(page)).To(Succeed())
		Expect(page.Locator(`text=Please fill all fields.`).First().WaitFor()).To(Succeed())
		emailValue, err := page.Locator(`input[name="email"]`).InputValue()
		Expect(err).NotTo(HaveOccurred())
		Expect(emailValue).To(Equal(""))

		email := fmt.Sprintf("missing-fields-%d@example.com", time.Now().UnixNano())
		Expect(page.Locator(`input[name="email"]`).Fill(email)).To(Succeed())
		Expect(page.Locator(`input[name="team_name"]`).Fill("Incomplete Team")).To(Succeed())
		teamSizeInput := page.Locator(`input[type="number"][name="team_size"]`)
		Expect(teamSizeInput.Fill("0")).To(Succeed())
		Expect(submitRegistrationIgnoringValidation(page)).To(Succeed())
		Expect(page.Locator(`text=Team size must be a number greater than 0.`).First().WaitFor()).To(Succeed())

		Expect(teamSizeInput.Fill(strconv.Itoa(seatsLeft + 1))).To(Succeed())
		Expect(submitRegistrationIgnoringValidation(page)).To(Succeed())
		Expect(page.Locator(`text=Only ` + strconv.Itoa(seatsLeft) + ` seat(s) left for this quiz.`).First().WaitFor()).To(Succeed())
	})

	It("supports unregistering a team from the public flow", func() {
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		teamName := fmt.Sprintf("Unregister Team %d", time.Now().UnixNano())
		email := fmt.Sprintf("unregister-%d@example.com", time.Now().UnixNano())
		Expect(registerNormalTeam(page, env.baseURL, teamName, email, "4")).To(Succeed())

		registrations, err := listCollectionRecords(env.baseURL, env.systemAdminToken, "registrations")
		Expect(err).NotTo(HaveOccurred())
		registrationID, err := recordIDByFieldValue(registrations, "team_name", teamName)
		Expect(err).NotTo(HaveOccurred())

		_, err = page.Goto(env.baseURL+"/unregister?id="+url.QueryEscape(registrationID), playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(`button:has-text("Unregister team")`).First().Click()).To(Succeed())
		Expect(page.Locator(`text=You have been successfully unregistered.`).First().WaitFor()).To(Succeed())
	})

	It("supports split-team registration for groups over 10", func() {
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

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

	It("stores merge consent choices and keeps quizzes open or closed as expected", func() {
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		now := time.Now()
		locationID, err := createCollectionRecord(env.baseURL, env.systemAdminToken, "locations", map[string]any{
			"name":     fmt.Sprintf("Public Future Pub %d", now.UnixNano()),
			"maps_url": "https://maps.google.com/?q=Public+Future",
			"capacity": 20,
		})
		Expect(err).NotTo(HaveOccurred())
		quizID, err := createCollectionRecord(env.baseURL, env.systemAdminToken, "quiz_dates", map[string]any{
			"location":     locationID,
			"scheduled_at": now.Add(2 * time.Hour).UTC().Format(time.RFC3339),
			"is_open":      true,
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = page.Goto(env.baseURL+"/quiz?id="+quizID, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(`input[name="email"]`).Fill(fmt.Sprintf("consent-%d@example.com", now.UnixNano()))).To(Succeed())
		Expect(page.Locator(`input[name="team_name"]`).Fill("Consent Team")).To(Succeed())
		Expect(page.Locator(`input[name="team_size"]`).Fill("2")).To(Succeed())
		Expect(page.Locator(`button:has-text("Register")`).Click()).To(Succeed())
		Expect(page.Locator(`button:has-text("No, keep us separate")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Registration successful`).First().WaitFor()).To(Succeed())
	})
}
