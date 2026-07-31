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
		Expect(response.Status()).To(Equal(400))

		bodyText, err := page.Locator("body").TextContent()
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.ToLower(bodyText)).To(ContainSubstring("quiz id is required"))

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
		Expect(closedResponse.Status()).To(Equal(400))

		bodyText, err = page.Locator("body").TextContent()
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.ToLower(bodyText)).To(ContainSubstring("failed to load quiz"))
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
