package e2e_tests

import (
	"io"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mxschmitt/playwright-go"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func defineFunctionalityTests(env *playwrightE2EEnv) {
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

			Expect(page.Locator(`header details > summary`).Click()).To(Succeed())
			Expect(page.Locator(`a[href="/"]`).First().WaitFor()).To(Succeed())
			Expect(page.Locator(`a[href="/about"]`).First().WaitFor()).To(Succeed())
			Expect(page.Locator(`a[href="/contact"]`).First().WaitFor()).To(Succeed())
			Expect(page.Locator(`a[href="/privacy"]`).First().WaitFor()).To(Succeed())
			Expect(page.Locator(`a[href="/quiz-admin"]`).First().WaitFor()).To(Succeed())
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
		Expect(lowerBody).To(ContainSubstring("404"))
		Expect(lowerBody).To(ContainSubstring("gopher"))
		Expect(lowerBody).To(ContainSubstring("beer"))

		_, err = page.Goto(env.baseURL, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		lastQuizHref, err := lastQuizHref(page)
		Expect(err).NotTo(HaveOccurred())
		lastQuizID, err := quizIDFromHref(lastQuizHref)
		Expect(err).NotTo(HaveOccurred())

		Expect(updateCollectionRecord(env.baseURL, env.systemAdminToken, "quiz_dates", lastQuizID, map[string]any{"is_open": false})).To(Succeed())

		closedResponse, err := page.Goto(env.baseURL+"/quiz?id="+url.QueryEscape(lastQuizID), playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(closedResponse).NotTo(BeNil())
		Expect(closedResponse.Status()).To(Equal(404))

		bodyText, err = page.Locator("body").TextContent()
		Expect(err).NotTo(HaveOccurred())
		lowerBody = strings.ToLower(bodyText)
		Expect(lowerBody).To(ContainSubstring("404"))
		Expect(lowerBody).To(ContainSubstring("gopher"))
		Expect(lowerBody).To(ContainSubstring("beer"))
	})

	It("validates registration input and seat capacity", func() {
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		quizURL, err := firstQuizURL(page, env.baseURL)
		Expect(err).NotTo(HaveOccurred())
		_, err = page.Goto(quizURL, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())

		seatsLeft, err := quizSeatsLeft(page)
		Expect(err).NotTo(HaveOccurred())

		Expect(submitRegistrationIgnoringValidation(page)).To(Succeed())
		Expect(page.Locator(`text=Please fill all fields.`).First().WaitFor()).To(Succeed())
		emailValue, err := page.Locator(`input[name="email"]`).InputValue()
		Expect(err).NotTo(HaveOccurred())
		Expect(emailValue).To(Equal(""))
		teamNameValue, err := page.Locator(`input[name="team_name"]`).InputValue()
		Expect(err).NotTo(HaveOccurred())
		Expect(teamNameValue).To(Equal(""))

		email := fmt.Sprintf("missing-fields-%d@example.com", time.Now().UnixNano())
		Expect(page.Locator(`input[name="email"]`).Fill(email)).To(Succeed())
		Expect(page.Locator(`input[name="team_name"]`).Fill("Incomplete Team")).To(Succeed())

		teamSizeInput := page.Locator(`input[type="number"][name="team_size"]`)
		Expect(teamSizeInput.Fill("0")).To(Succeed())
		Expect(submitRegistrationIgnoringValidation(page)).To(Succeed())
		Expect(page.Locator(`text=Team size must be a number greater than 0.`).First().WaitFor()).To(Succeed())
		emailValue, err = page.Locator(`input[name="email"]`).InputValue()
		Expect(err).NotTo(HaveOccurred())
		Expect(emailValue).To(Equal(email))
		teamNameValue, err = page.Locator(`input[name="team_name"]`).InputValue()
		Expect(err).NotTo(HaveOccurred())
		Expect(teamNameValue).To(Equal("Incomplete Team"))
		teamSizeValue, err := teamSizeInput.InputValue()
		Expect(err).NotTo(HaveOccurred())
		Expect(teamSizeValue).To(Equal("0"))

		Expect(teamSizeInput.Fill(strconv.Itoa(seatsLeft + 1))).To(Succeed())
		Expect(submitRegistrationIgnoringValidation(page)).To(Succeed())
		Expect(page.Locator(`text=Only ` + strconv.Itoa(seatsLeft) + ` seat(s) left for this quiz.`).First().WaitFor()).To(Succeed())
		teamSizeValue, err = teamSizeInput.InputValue()
		Expect(err).NotTo(HaveOccurred())
		Expect(teamSizeValue).To(Equal(strconv.Itoa(seatsLeft + 1)))
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

		Expect(page.Locator(`h2:has-text("Unregister Team")`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`dd:has-text("` + teamName + `")`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`dd:has-text("4")`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`dd:has-text("` + email + `")`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`button:has-text("Unregister team")`).First().WaitFor()).To(Succeed())

		Expect(page.Locator(`button:has-text("Unregister team")`).Click()).To(Succeed())
		Expect(page.Locator(`text=You have been successfully unregistered.`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`a:has-text("Back to start page")`).First().WaitFor()).To(Succeed())

		registrations, err = listCollectionRecords(env.baseURL, env.systemAdminToken, "registrations")
		Expect(err).NotTo(HaveOccurred())
		_, err = recordIDByFieldValue(registrations, "team_name", teamName)
		Expect(err).To(HaveOccurred())
	})

	It("logs out quiz admins and protects admin routes", func() {
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		Expect(loginQuizAdmin(page, env.baseURL, env.quizAdminEmail, env.quizAdminPassword)).To(Succeed())

		Expect(page.Locator(`header details > summary`).Click()).To(Succeed())
		Expect(page.Locator(`button:has-text("Logout")`).Click()).To(Succeed())
		Expect(page.Locator(`h2:has-text("Quiz Admin Login")`).First().WaitFor()).To(Succeed())

		_, err = page.Goto(env.baseURL+"/quiz-admin/collections/locations", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(`h2:has-text("Quiz Admin Login")`).First().WaitFor()).To(Succeed())
	})

	It("allows an end-user to register a normal team", func() {
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		Expect(registerNormalTeam(page, env.baseURL, "Playwright Team", fmt.Sprintf("playwright-%d@example.com", time.Now().UnixNano()), "4")).To(Succeed())
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

	It("shows split prompt details with balanced split sizes", func() {
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		for _, tc := range []struct {
			teamSize       string
			expectedPrompt string
			expectedInputs int
		}{
			{
				teamSize:       "11",
				expectedPrompt: "Teams can have at most 10 people. Your group can be split into 2 teams with sizes: 6, 5. Do you want to continue?",
				expectedInputs: 2,
			},
			{
				teamSize:       "20",
				expectedPrompt: "Teams can have at most 10 people. Your group can be split into 2 teams with sizes: 10, 10. Do you want to continue?",
				expectedInputs: 2,
			},
			{
				teamSize:       "21",
				expectedPrompt: "Teams can have at most 10 people. Your group can be split into 3 teams with sizes: 7, 7, 7. Do you want to continue?",
				expectedInputs: 3,
			},
		} {
			Expect(openSplitPrompt(page, env.baseURL, tc.teamSize)).To(Succeed())
			Expect(page.Locator(`text=` + tc.expectedPrompt).First().WaitFor()).To(Succeed())

			inputs := page.Locator(`input[name^="split_team_name_"]`)
			count, err := inputs.Count()
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(tc.expectedInputs))

			for i := 0; i < tc.expectedInputs; i++ {
				value, err := inputs.Nth(i).InputValue()
				Expect(err).NotTo(HaveOccurred())
				Expect(value).NotTo(BeEmpty())
			}
		}
	})

	It("requires every split team name before submission", func() {
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		Expect(openSplitPrompt(page, env.baseURL, "11")).To(Succeed())
		Expect(page.Locator(`input[name="split_team_name_0"]`).Fill("Split A")).To(Succeed())
		Expect(page.Locator(`input[name="split_team_name_1"]`).Fill("")).To(Succeed())
		Expect(submitRegistrationIgnoringValidation(page)).To(Succeed())

		Expect(page.Locator(`text=Please provide a name for every split team.`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`text=Teams can have at most 10 people.`).First().WaitFor()).To(Succeed())
		firstValue, err := page.Locator(`input[name="split_team_name_0"]`).InputValue()
		Expect(err).NotTo(HaveOccurred())
		Expect(firstValue).To(Equal("Split A"))
		secondValue, err := page.Locator(`input[name="split_team_name_1"]`).InputValue()
		Expect(err).NotTo(HaveOccurred())
		Expect(secondValue).To(Equal(""))
	})

	It("resets the split flow after a successful split registration", func() {
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		Expect(openSplitPrompt(page, env.baseURL, "11")).To(Succeed())
		startingSeatsLeft, err := quizSeatsLeft(page)
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(`input[name="split_team_name_0"]`).Fill("Split A")).To(Succeed())
		Expect(page.Locator(`input[name="split_team_name_1"]`).Fill("Split B")).To(Succeed())
		Expect(page.Locator(`button:has-text("Yes, split and register")`).Click()).To(Succeed())
		Expect(page.Locator(`#registration-panel >> text=Registration successful`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`text=Seats left: ` + strconv.Itoa(startingSeatsLeft-11)).First().WaitFor()).To(Succeed())

		tileURL, err := firstQuizURL(page, env.baseURL)
		Expect(err).NotTo(HaveOccurred())
		_, err = page.Goto(tileURL, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		emailValue, err := page.Locator(`input[name="email"]`).InputValue()
		Expect(err).NotTo(HaveOccurred())
		Expect(emailValue).To(Equal(""))
		teamNameValue, err := page.Locator(`input[name="team_name"]`).InputValue()
		Expect(err).NotTo(HaveOccurred())
		Expect(teamNameValue).To(Equal(""))
		teamSizeValue, err := page.Locator(`input[type="number"][name="team_size"]`).InputValue()
		Expect(err).NotTo(HaveOccurred())
		Expect(teamSizeValue).To(Equal("1"))
	})

	It("supports merge-consent prompt for small teams", func() {
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		Expect(registerSmallTeamWithMergeConsent(page, env.baseURL, "Playwright Tiny", fmt.Sprintf("tiny-%d@example.com", time.Now().UnixNano()))).To(Succeed())
	})

	It("supports quiz admin CRUD for locations and quiz dates", func() {
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
		Eventually(func() string {
			return page.URL()
		}).Should(ContainSubstring("/quiz-admin/collections/locations"))
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
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

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

	It("stores merge consent choices and keeps quizzes open or closed as expected", func() {
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		now := time.Now()
		futureLocationName := fmt.Sprintf("Playwright Future Pub %d", now.UnixNano())
		nearLocationName := fmt.Sprintf("Playwright Near Pub %d", now.UnixNano()+1)
		fullLocationName := fmt.Sprintf("Playwright Full Pub %d", now.UnixNano()+2)

		futureLocationID, err := createCollectionRecord(env.baseURL, env.systemAdminToken, "locations", map[string]any{
			"name":     futureLocationName,
			"maps_url": "https://maps.google.com/?q=" + url.QueryEscape(futureLocationName),
			"capacity": 20,
		})
		Expect(err).NotTo(HaveOccurred())
		nearLocationID, err := createCollectionRecord(env.baseURL, env.systemAdminToken, "locations", map[string]any{
			"name":     nearLocationName,
			"maps_url": "https://maps.google.com/?q=" + url.QueryEscape(nearLocationName),
			"capacity": 20,
		})
		Expect(err).NotTo(HaveOccurred())
		fullLocationID, err := createCollectionRecord(env.baseURL, env.systemAdminToken, "locations", map[string]any{
			"name":     fullLocationName,
			"maps_url": "https://maps.google.com/?q=" + url.QueryEscape(fullLocationName),
			"capacity": 5,
		})
		Expect(err).NotTo(HaveOccurred())

		futureQuizID, err := createCollectionRecord(env.baseURL, env.systemAdminToken, "quiz_dates", map[string]any{
			"location":     futureLocationID,
			"scheduled_at": now.Add(2 * time.Hour).UTC().Format(time.RFC3339),
			"is_open":      true,
		})
		Expect(err).NotTo(HaveOccurred())
		nearQuizID, err := createCollectionRecord(env.baseURL, env.systemAdminToken, "quiz_dates", map[string]any{
			"location":     nearLocationID,
			"scheduled_at": now.Add(30 * time.Minute).UTC().Format(time.RFC3339),
			"is_open":      true,
		})
		Expect(err).NotTo(HaveOccurred())
		fullQuizID, err := createCollectionRecord(env.baseURL, env.systemAdminToken, "quiz_dates", map[string]any{
			"location":     fullLocationID,
			"scheduled_at": now.Add(2 * time.Hour).UTC().Format(time.RFC3339),
			"is_open":      true,
		})
		Expect(err).NotTo(HaveOccurred())

		futureQuizURL := env.baseURL + "/quiz?id=" + futureQuizID
		nearQuizURL := env.baseURL + "/quiz?id=" + nearQuizID
		fullQuizURL := env.baseURL + "/quiz?id=" + fullQuizID

		consentTeamName := fmt.Sprintf("Consent Team %d", now.UnixNano())
		consentEmail := fmt.Sprintf("consent-%d@example.com", now.UnixNano())
		_, err = page.Goto(futureQuizURL, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(`input[name="email"]`).Fill(consentEmail)).To(Succeed())
		Expect(page.Locator(`input[name="team_name"]`).Fill(consentTeamName)).To(Succeed())
		Expect(page.Locator(`input[name="team_size"]`).Fill("2")).To(Succeed())
		Expect(page.Locator(`button:has-text("Register")`).Click()).To(Succeed())
		Expect(page.Locator(`button:has-text("No, keep us separate")`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`button:has-text("No, keep us separate")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Registration successful`).First().WaitFor()).To(Succeed())

		records, err := listCollectionRecords(env.baseURL, env.systemAdminToken, "registrations")
		Expect(err).NotTo(HaveOccurred())
		consentRecordID, err := recordIDByFieldValue(records, "team_name", consentTeamName)
		Expect(err).NotTo(HaveOccurred())
		consentRecord := func() map[string]any {
			for _, record := range records {
				if recordID, _ := record["id"].(string); recordID == consentRecordID {
					return record
				}
			}
			return nil
		}()
		Expect(consentRecord).NotTo(BeNil())
		Expect(consentRecord["willing_to_merge"]).To(Equal(false))

		response, err := http.PostForm(env.baseURL+"/register", url.Values{
			"quiz_id":        {futureQuizID},
			"email":          {fmt.Sprintf("invalid-merge-%d@example.com", now.UnixNano())},
			"team_name":      {"Invalid Merge Team"},
			"team_size":      {"2"},
			"confirm_merge":  {"maybe"},
		})
		Expect(err).NotTo(HaveOccurred())
		body, err := io.ReadAll(response.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect(response.Body.Close()).To(Succeed())
		Expect(string(body)).To(ContainSubstring("Please choose whether your team can be merged with another small team."))

		_, err = page.Goto(nearQuizURL, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		nearCount, err := page.Locator(`text=` + nearLocationName).Count()
		Expect(err).NotTo(HaveOccurred())
		Expect(nearCount).To(Equal(0))

		_, err = page.Goto(futureQuizURL, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(`text=` + futureLocationName).First().WaitFor()).To(Succeed())

		_, err = page.Goto(fullQuizURL, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(`input[name="email"]`).Fill(fmt.Sprintf("full-%d@example.com", now.UnixNano()))).To(Succeed())
		Expect(page.Locator(`input[name="team_name"]`).Fill("Full Team")).To(Succeed())
		Expect(page.Locator(`input[name="team_size"]`).Fill("5")).To(Succeed())
		Expect(page.Locator(`button:has-text("Register")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Registration successful`).First().WaitFor()).To(Succeed())

		_, err = page.Goto(fullQuizURL, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(`text=404`).First().WaitFor()).To(Succeed())
		fullCount, err := page.Locator(`text=` + fullLocationName).Count()
		Expect(err).NotTo(HaveOccurred())
		Expect(fullCount).To(Equal(0))

		_, err = page.Goto(env.baseURL, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(`text=` + futureLocationName).First().WaitFor()).To(Succeed())
		nearHomeCount, err := page.Locator(`text=` + nearLocationName).Count()
		Expect(err).NotTo(HaveOccurred())
		Expect(nearHomeCount).To(Equal(0))
		fullHomeCount, err := page.Locator(`text=` + fullLocationName).Count()
		Expect(err).NotTo(HaveOccurred())
		Expect(fullHomeCount).To(Equal(0))
	})

	It("covers quiz admin collection edge states and readable labels", func() {
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		_, err = page.Goto(env.baseURL+"/quiz-admin/collections/locations", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(loginQuizAdmin(page, env.baseURL, env.quizAdminEmail, env.quizAdminPassword)).To(Succeed())
		_, err = page.Goto(env.baseURL+"/quiz-admin/collections/locations", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())

		Expect(page.Locator(`#admin-collection-sidebar a:has-text("quiz_admins")`).Count()).To(Equal(0))
		Expect(page.Locator(`#admin-collection-sidebar a:has-text("locations")`).Count()).To(Equal(1))
		Expect(page.Locator(`#admin-collection-sidebar a:has-text("quiz_dates")`).Count()).To(Equal(1))
		Expect(page.Locator(`#admin-collection-sidebar a:has-text("registrations")`).Count()).To(Equal(1))

		locationName := fmt.Sprintf("Playwright Edge Pub %d", time.Now().UnixNano())
		locationMapURL := "https://maps.google.com/?q=" + url.QueryEscape(locationName)

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
		editForm := visibleEditModal.Locator(`form`)
		_, err = editForm.Evaluate(`form => { form.noValidate = true; }`, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(visibleEditModal.Locator(`input[name="field_name"]`).Fill(locationName)).To(Succeed())
		Expect(visibleEditModal.Locator(`input[name="field_maps_url"]`).Fill(locationMapURL)).To(Succeed())
		Expect(visibleEditModal.Locator(`input[name="field_capacity"]`).Fill("42")).To(Succeed())
		_, err = page.Evaluate(`() => {
			const input = document.querySelector('input[name="field_capacity"]');
			input.type = 'text';
			input.value = 'oops';
		}`)
		Expect(err).NotTo(HaveOccurred())
		Expect(visibleEditModal.Locator(`button:has-text("Create")`).Click()).To(Succeed())
		Expect(page.Locator(`text=capacity must be a valid number`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`#admin-edit-modal`).First().WaitFor()).To(Succeed())

		Expect(visibleEditModal.Locator(`a:has-text("Close")`).Click()).To(Succeed())
		Eventually(func() bool {
			visible, err := page.Locator(`#admin-edit-modal`).First().IsVisible()
			Expect(err).NotTo(HaveOccurred())
			return visible
		}).Should(BeFalse())

		Expect(page.Locator(`a:has-text("Add entry")`).Click()).To(Succeed())
		visibleEditModal = page.Locator(`#admin-edit-modal:not(.hidden)`)
		Expect(visibleEditModal.First().WaitFor()).To(Succeed())
		editForm = visibleEditModal.Locator(`form`)
		Expect(visibleEditModal.Locator(`input[name="field_name"]`).Fill(locationName)).To(Succeed())
		Expect(visibleEditModal.Locator(`input[name="field_maps_url"]`).Fill(locationMapURL)).To(Succeed())
		Expect(visibleEditModal.Locator(`input[name="field_capacity"]`).Fill("42")).To(Succeed())
		Expect(visibleEditModal.Locator(`button:has-text("Create")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Entry created.`).First().WaitFor()).To(Succeed())
		Eventually(func() bool {
			visible, err := page.Locator(`#admin-edit-modal`).First().IsVisible()
			Expect(err).NotTo(HaveOccurred())
			return visible
		}).Should(BeFalse())

		locationRow := page.Locator(fmt.Sprintf(`tr:has-text("%s") a:has-text("%s")`, locationName, locationName)).First()
		Expect(locationRow.Click()).To(Succeed())
		visibleEditModal = page.Locator(`#admin-edit-modal:not(.hidden)`)
		Expect(visibleEditModal.First().WaitFor()).To(Succeed())
		Expect(page.Locator(`text=Record ID:`).First().WaitFor()).To(Succeed())
		Expect(visibleEditModal.Locator(`input[name="field_capacity"]`).Fill("43")).To(Succeed())
		Expect(visibleEditModal.Locator(`button:has-text("Save changes")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Entry updated.`).First().WaitFor()).To(Succeed())
		Eventually(func() string {
			return page.URL()
		}).Should(ContainSubstring("/quiz-admin/collections/locations"))

		records, err := listCollectionRecords(env.baseURL, env.systemAdminToken, "locations")
		Expect(err).NotTo(HaveOccurred())
		locationRowID, err := recordIDByFieldValue(records, "name", locationName)
		Expect(err).NotTo(HaveOccurred())
		quizDateID, err := createCollectionRecord(env.baseURL, env.systemAdminToken, "quiz_dates", map[string]any{
			"location":     locationRowID,
			"scheduled_at": time.Now().Add(3 * time.Hour).UTC().Format(time.RFC3339),
			"is_open":      true,
		})
		Expect(err).NotTo(HaveOccurred())
		_, err = page.Goto(env.baseURL+"/quiz-admin/collections/quiz_dates", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(`a:has-text("Add entry")`).Click()).To(Succeed())
		quizDateModal := page.Locator(`#admin-edit-modal`)
		Expect(quizDateModal.First().WaitFor()).To(Succeed())
		_, err = quizDateModal.Locator(`select[name="field_location"]`).SelectOption(playwright.SelectOptionValues{Labels: playwright.StringSlice(locationName)})
		Expect(err).NotTo(HaveOccurred())
		Expect(quizDateModal.Locator(`input[name="field_scheduled_at"]`).Fill(time.Now().Add(4 * time.Hour).Format("2006-01-02T15:04"))).To(Succeed())
		Expect(quizDateModal.Locator(`button:has-text("Create")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Entry created.`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s")`, locationName)).First().WaitFor()).To(Succeed())

		registrationName := fmt.Sprintf("Cascade Team %d", time.Now().UnixNano())
		_, err = createCollectionRecord(env.baseURL, env.systemAdminToken, "registrations", map[string]any{
			"quiz":            quizDateID,
			"email":           fmt.Sprintf("cascade-%d@example.com", time.Now().UnixNano()),
			"team_name":       registrationName,
			"team_size":       3,
			"willing_to_merge": true,
		})
		Expect(err).NotTo(HaveOccurred())
		_, err = page.Goto(env.baseURL+"/quiz-admin/collections/quiz_dates", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s") input[name="delete_ids"]`, locationName)).First().Check()).To(Succeed())
		Expect(page.Locator(`button:has-text("Delete selected")`).Click()).To(Succeed())
		Expect(page.Locator(`#admin-delete-modal button:has-text("Delete")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Deleted 1 entrie(s).`).First().WaitFor()).To(Succeed())

		_, err = page.Goto(env.baseURL+"/quiz-admin/collections/registrations", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s")`, registrationName)).Count()).To(Equal(0))

		_, err = page.Goto(env.baseURL+"/quiz-admin/collections/locations", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(`button:has-text("Delete selected")`).Click()).To(Succeed())
		Expect(page.Locator(`#admin-delete-modal button:has-text("Delete")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Select at least one entry to delete.`).First().WaitFor()).To(Succeed())
	})

	It("covers registration merge selection, preview, and validation", func() {
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		locationName := fmt.Sprintf("Playwright Merge Pub %d", time.Now().UnixNano())
		locationID, err := createCollectionRecord(env.baseURL, env.systemAdminToken, "locations", map[string]any{
			"name":     locationName,
			"maps_url": "https://maps.google.com/?q=" + url.QueryEscape(locationName),
			"capacity": 40,
		})
		Expect(err).NotTo(HaveOccurred())
		quizAID, err := createCollectionRecord(env.baseURL, env.systemAdminToken, "quiz_dates", map[string]any{
			"location":     locationID,
			"scheduled_at": time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339),
			"is_open":      true,
		})
		Expect(err).NotTo(HaveOccurred())
		quizBID, err := createCollectionRecord(env.baseURL, env.systemAdminToken, "quiz_dates", map[string]any{
			"location":     locationID,
			"scheduled_at": time.Now().Add(3 * time.Hour).UTC().Format(time.RFC3339),
			"is_open":      true,
		})
		Expect(err).NotTo(HaveOccurred())

		teamA1 := fmt.Sprintf("Merge Alpha %d", time.Now().UnixNano())
		teamA2 := fmt.Sprintf("Merge Beta %d", time.Now().UnixNano())
		teamB := fmt.Sprintf("Merge Gamma %d", time.Now().UnixNano())
		teamNoMerge := fmt.Sprintf("Merge No %d", time.Now().UnixNano())

		teamA1Email := fmt.Sprintf("alpha-%d@example.com", time.Now().UnixNano())
		teamA2Email := fmt.Sprintf("beta-%d@example.com", time.Now().UnixNano())
		teamBEmail := fmt.Sprintf("gamma-%d@example.com", time.Now().UnixNano())
		teamNoMergeEmail := fmt.Sprintf("no-%d@example.com", time.Now().UnixNano())

		_, err = createCollectionRecord(env.baseURL, env.systemAdminToken, "registrations", map[string]any{
			"quiz":             quizAID,
			"email":            teamA1Email,
			"team_name":        teamA1,
			"team_size":        2,
			"willing_to_merge": true,
		})
		Expect(err).NotTo(HaveOccurred())
		_, err = createCollectionRecord(env.baseURL, env.systemAdminToken, "registrations", map[string]any{
			"quiz":             quizAID,
			"email":            teamA2Email,
			"team_name":        teamA2,
			"team_size":        3,
			"willing_to_merge": true,
		})
		Expect(err).NotTo(HaveOccurred())
		_, err = createCollectionRecord(env.baseURL, env.systemAdminToken, "registrations", map[string]any{
			"quiz":             quizBID,
			"email":            teamBEmail,
			"team_name":        teamB,
			"team_size":        4,
			"willing_to_merge": true,
		})
		Expect(err).NotTo(HaveOccurred())
		_, err = createCollectionRecord(env.baseURL, env.systemAdminToken, "registrations", map[string]any{
			"quiz":             quizAID,
			"email":            teamNoMergeEmail,
			"team_name":        teamNoMerge,
			"team_size":        2,
			"willing_to_merge": false,
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(loginQuizAdmin(page, env.baseURL, env.quizAdminEmail, env.quizAdminPassword)).To(Succeed())
		_, err = page.Goto(env.baseURL+"/quiz-admin/collections/registrations", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() bool {
			disabled, err := page.Locator(`#merge-entries-button`).IsDisabled()
			Expect(err).NotTo(HaveOccurred())
			return disabled
		}).Should(BeTrue())
		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s") input[name="delete_ids"]`, teamA1)).First().Check()).To(Succeed())
		Eventually(func() bool {
			disabled, err := page.Locator(`#merge-entries-button`).IsDisabled()
			Expect(err).NotTo(HaveOccurred())
			return disabled
		}).Should(BeTrue())

		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s") input[name="delete_ids"]`, teamNoMerge)).First().Check()).To(Succeed())
		Eventually(func() bool {
			disabled, err := page.Locator(`#merge-entries-button`).IsDisabled()
			Expect(err).NotTo(HaveOccurred())
			return disabled
		}).Should(BeTrue())
		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s") input[name="delete_ids"]`, teamNoMerge)).First().Uncheck()).To(Succeed())

		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s") input[name="delete_ids"]`, teamB)).First().Check()).To(Succeed())
		records, err := listCollectionRecords(env.baseURL, env.systemAdminToken, "registrations")
		Expect(err).NotTo(HaveOccurred())
		teamA1RecordID, err := recordIDByFieldValue(records, "team_name", teamA1)
		Expect(err).NotTo(HaveOccurred())
		teamBRecordID, err := recordIDByFieldValue(records, "team_name", teamB)
		Expect(err).NotTo(HaveOccurred())
		mergeErrorTextRaw, err := page.Evaluate(`async ids => {
			const params = new URLSearchParams();
			for (const id of ids) {
				params.append('delete_ids', id);
			}
			const response = await fetch('/quiz-admin/collections/registrations/merge/modal', {
				method: 'POST',
				headers: {
					'Content-Type': 'application/x-www-form-urlencoded',
					'HX-Request': 'true',
				},
				body: params,
			});
			return await response.text();
		}`, []string{teamA1RecordID, teamBRecordID})
		Expect(err).NotTo(HaveOccurred())
		Expect(fmt.Sprint(mergeErrorTextRaw)).To(ContainSubstring("all selected registrations must belong to the same quiz"))

		_, err = page.Goto(env.baseURL+"/quiz-admin/collections/registrations", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s") input[name="delete_ids"]`, teamA1)).First().Check()).To(Succeed())
		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s") input[name="delete_ids"]`, teamA2)).First().Check()).To(Succeed())
		Eventually(func() bool {
			disabled, err := page.Locator(`#merge-entries-button`).IsDisabled()
			Expect(err).NotTo(HaveOccurred())
			return disabled
		}).Should(BeFalse())

		Expect(page.Locator(`#merge-entries-button`).Click()).To(Succeed())
		Expect(page.Locator(`h3:has-text("Merge entries")`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`text=` + teamA1).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`text=` + teamA2).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`text=2 entries from`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`text=` + locationName).First().WaitFor()).To(Succeed())

		Expect(page.Locator(`#merge-entries-modal button:has-text("Close")`).Click()).To(Succeed())
		Eventually(func() bool {
			visible, err := page.Locator(`#merge-entries-modal`).First().IsVisible()
			Expect(err).NotTo(HaveOccurred())
			return visible
		}).Should(BeFalse())

		Expect(page.Locator(`#merge-entries-button`).Click()).To(Succeed())
		Expect(page.Locator(`#merge-entries-modal input[name="team_name"]`).Fill("")).To(Succeed())
		Expect(page.Locator(`#merge-entries-modal input[name="email"]`).Fill("")).To(Succeed())
		Expect(page.Locator(`#merge-entries-modal button:has-text("Merge selected teams")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Entries merged successfully into`).First().WaitFor()).To(Succeed())

		mergedRows := page.Locator(fmt.Sprintf(`tr:has-text("%s")`, teamA1))
		Expect(mergedRows.First().WaitFor()).To(Succeed())
		mergedText, err := mergedRows.First().TextContent()
		Expect(err).NotTo(HaveOccurred())
		Expect(mergedText).To(ContainSubstring(teamA1))
		Expect(mergedText).To(ContainSubstring(teamA2))
		Expect(strings.Contains(mergedText, teamA1Email) || strings.Contains(mergedText, teamA2Email)).To(BeTrue())

		_, err = page.Goto(env.baseURL+"/quiz-admin/collections/registrations", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(`button:has-text("Delete selected")`).Click()).To(Succeed())
		Expect(page.Locator(`#admin-delete-modal button:has-text("Delete")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Select at least one entry to delete.`).First().WaitFor()).To(Succeed())
	})

	It("shows an empty home state when all quizzes are closed", func() {
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		quizDates, err := listCollectionRecords(env.baseURL, env.systemAdminToken, "quiz_dates")
		Expect(err).NotTo(HaveOccurred())
		for _, record := range quizDates {
			recordID, ok := record["id"].(string)
			Expect(ok).To(BeTrue())
			Expect(updateCollectionRecord(env.baseURL, env.systemAdminToken, "quiz_dates", recordID, map[string]any{"is_open": false})).To(Succeed())
		}

		_, err = page.Goto(env.baseURL, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(`text=No upcoming quizzes yet.`).First().WaitFor()).To(Succeed())
		tileCount, err := page.Locator(`a[href^="/quiz?id="]`).Count()
		Expect(err).NotTo(HaveOccurred())
		Expect(tileCount).To(Equal(0))
	})

	It("shows a setup error when required collections are missing", func() {
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		Expect(deleteCollection(env.baseURL, env.systemAdminToken, "registrations")).To(Succeed())
		Expect(deleteCollection(env.baseURL, env.systemAdminToken, "quiz_dates")).To(Succeed())

		_, err = page.Goto(env.baseURL, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(`text=The database collections are not ready yet.`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`text=Import pb_schema.json in the PocketBase Admin UI, then add locations and quiz dates.`).First().WaitFor()).To(Succeed())
	})
}

func firstQuizURL(page playwright.Page, baseURL string) (string, error) {
	_, err := page.Goto(baseURL, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
	if err != nil {
		return "", err
	}
	return quizURLFromLocator(page.Locator(`a[href^="/quiz?id="]`).First(), baseURL)
}

func lastQuizHref(page playwright.Page) (string, error) {
	count, err := page.Locator(`a[href^="/quiz?id="]`).Count()
	if err != nil {
		return "", err
	}
	if count == 0 {
		return "", fmt.Errorf("no quiz links found")
	}
	href, err := page.Locator(`a[href^="/quiz?id="]`).Nth(count - 1).GetAttribute("href")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(href) == "" {
		return "", fmt.Errorf("last quiz link has empty href")
	}
	return href, nil
}

func quizURLFromLocator(locator playwright.Locator, baseURL string) (string, error) {
	href, err := locator.GetAttribute("href")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(href) == "" {
		return "", fmt.Errorf("quiz link has empty href")
	}
	return baseURL + href, nil
}

func quizIDFromHref(href string) (string, error) {
	parsed, err := url.Parse(href)
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(parsed.Query().Get("id"))
	if id == "" {
		return "", fmt.Errorf("quiz href %q does not contain an id", href)
	}
	return id, nil
}

func quizSeatsLeft(page playwright.Page) (int, error) {
	value, err := page.Locator(`#registration-panel p strong`).TextContent()
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(value))
}

func openSplitPrompt(page playwright.Page, baseURL, teamSize string) error {
	_, err := page.Goto(baseURL, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
	if err != nil {
		return err
	}
	if err := page.Locator(`a:has-text("Register team")`).First().Click(); err != nil {
		return err
	}
	if err := page.Locator(`input[name="email"]`).Fill(fmt.Sprintf("split-%s-%d@example.com", teamSize, time.Now().UnixNano())); err != nil {
		return err
	}
	if err := page.Locator(`input[name="team_name"]`).Fill("Split Team " + teamSize); err != nil {
		return err
	}
	if err := page.Locator(`input[name="team_size"]`).Fill(teamSize); err != nil {
		return err
	}
	if err := page.Locator(`button:has-text("Register")`).Click(); err != nil {
		return err
	}
	return page.Locator(`text=Teams can have at most 10 people`).First().WaitFor()
}

func submitRegistrationIgnoringValidation(page playwright.Page) error {
	_, err := page.Evaluate(`() => {
		const form = document.querySelector('#registration-panel form');
		if (!form) {
			throw new Error('registration form not found');
		}
		form.noValidate = true;
		const button = form.querySelector('button[type="submit"]');
		if (!button) {
			throw new Error('registration submit button not found');
		}
		button.click();
	}`)
	return err
}

func recordIDByFieldValue(records []map[string]any, field, value string) (string, error) {
	for _, record := range records {
		rawValue, ok := record[field]
		if !ok {
			continue
		}
		if strings.TrimSpace(fmt.Sprint(rawValue)) == value {
			id, ok := record["id"].(string)
			if !ok || strings.TrimSpace(id) == "" {
				return "", fmt.Errorf("record matched %s=%q but had no id", field, value)
			}
			return id, nil
		}
	}
	return "", fmt.Errorf("record with %s=%q not found", field, value)
}
