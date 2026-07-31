package e2e_tests

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/mxschmitt/playwright-go"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func definePublicTests(env *playwrightE2EEnv) {
	It("renders the public home page with quiz tiles and the navigation menu", func() {
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		_, err = page.Goto(env.baseURL, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())

		title, err := page.Title()
		Expect(err).NotTo(HaveOccurred())
		Expect(title).To(Equal("Pub Quiz Dates"))
		headerTitle, err := page.Locator(`header h1`).TextContent()
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.TrimSpace(headerTitle)).To(Equal("Pub Quiz Signup"))
		Expect(page.Locator(`h2:has-text("Available quiz dates")`).First().WaitFor()).To(Succeed())

		tiles := page.Locator(`a[href^="/quiz?id="]`)
		tileCount, err := tiles.Count()
		Expect(err).NotTo(HaveOccurred())
		Expect(tileCount).To(BeNumerically(">", 0))
		Expect(tiles.First().Locator(`text=Register team`).First().WaitFor()).To(Succeed())

		Expect(page.Locator(`header details > summary`).Click()).To(Succeed())
		Expect(page.Locator(`a:has-text("Home")`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`a:has-text("About Us")`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`a:has-text("Contact Us")`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`a:has-text("Privacy Policy")`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`a:has-text("Quiz Admin")`).First().WaitFor()).To(Succeed())
	})

	It("opens a quiz tile through htmx navigation", func() {
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		_, err = page.Goto(env.baseURL, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())

		tile := page.Locator(`a[href^="/quiz?id="]`).First()
		href, err := tile.GetAttribute("href")
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.TrimSpace(href)).NotTo(BeEmpty())

		Expect(tile.Click()).To(Succeed())
		Expect(page.Locator(`#quiz-content #registration-panel`).First().WaitFor()).To(Succeed())
		Expect(page.URL()).To(Equal(env.baseURL + href))
	})

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

	It("registers a valid team and updates the seats left counter", func() {
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		_, err = page.Goto(env.baseURL, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())

		tile := page.Locator(`a[href^="/quiz?id="]`).First()
		href, err := tile.GetAttribute("href")
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.TrimSpace(href)).NotTo(BeEmpty())
		_, err = page.Goto(env.baseURL+href, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())

		seatsBeforeText, err := page.Locator(`#registration-panel p strong`).TextContent()
		Expect(err).NotTo(HaveOccurred())
		seatsBefore, err := strconv.Atoi(strings.TrimSpace(seatsBeforeText))
		Expect(err).NotTo(HaveOccurred())

		teamName := fmt.Sprintf("Happy Team %d", time.Now().UnixNano())
		email := fmt.Sprintf("happy-%d@example.com", time.Now().UnixNano())
		teamSize := 4

		Expect(page.Locator(`input[name="email"]`).Fill(email)).To(Succeed())
		Expect(page.Locator(`input[name="team_name"]`).Fill(teamName)).To(Succeed())
		Expect(page.Locator(`input[name="team_size"]`).Fill(strconv.Itoa(teamSize))).To(Succeed())
		Expect(page.Locator(`button:has-text("Register")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Registration successful. See you at the quiz!`).First().WaitFor()).To(Succeed())

		seatsAfterText, err := page.Locator(`#registration-panel p strong`).TextContent()
		Expect(err).NotTo(HaveOccurred())
		seatsAfter, err := strconv.Atoi(strings.TrimSpace(seatsAfterText))
		Expect(err).NotTo(HaveOccurred())
		Expect(seatsAfter).To(Equal(seatsBefore - teamSize))

		Expect(page.Locator(`input[name="email"]`).Count()).To(Equal(0))
		Expect(page.Locator(`input[name="team_name"]`).Count()).To(Equal(0))
		Expect(page.Locator(`input[name="team_size"]`).Count()).To(Equal(0))
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
		teamNameText, err := page.Locator(`dt:has-text("Team name") + dd`).First().TextContent()
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.TrimSpace(teamNameText)).To(Equal(teamName))
		teamSizeText, err := page.Locator(`dt:has-text("Team size") + dd`).First().TextContent()
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.TrimSpace(teamSizeText)).To(Equal("4"))
		emailText, err := page.Locator(`dt:has-text("Registration email") + dd`).First().TextContent()
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.TrimSpace(emailText)).To(Equal(email))
		Expect(page.Locator(`button:has-text("Unregister team")`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`button:has-text("Unregister team")`).First().Click()).To(Succeed())
		Expect(page.Locator(`text=You have been successfully unregistered.`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`a:has-text("Back to start page")`).First().WaitFor()).To(Succeed())

		response, err := page.Goto(env.baseURL+"/unregister?id="+url.QueryEscape(registrationID), playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(response).NotTo(BeNil())
		Expect(response.Status()).To(Equal(404))
		bodyText, err := page.Locator(`body`).TextContent()
		Expect(err).NotTo(HaveOccurred())
		Expect(bodyText).To(ContainSubstring("Registration not found."))
	})

	It("supports split-team registration for groups over 10", func() {
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		_, err = page.Goto(env.baseURL, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(`a:has-text("Register team")`).First().Click()).To(Succeed())
		initialSeatsText, err := page.Locator(`#registration-panel p strong`).TextContent()
		Expect(err).NotTo(HaveOccurred())
		initialSeats, err := strconv.Atoi(strings.TrimSpace(initialSeatsText))
		Expect(err).NotTo(HaveOccurred())

		Expect(page.Locator(`input[name="email"]`).Fill(fmt.Sprintf("split-%d@example.com", time.Now().UnixNano()))).To(Succeed())
		Expect(page.Locator(`input[name="team_name"]`).Fill("Playwright Split")).To(Succeed())
		Expect(page.Locator(`input[name="team_size"]`).Fill("11")).To(Succeed())
		Expect(page.Locator(`button:has-text("Register")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Teams can have at most 10 people`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`text=Your group can be split into 2 teams with sizes: 6, 5.`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`input[name="split_team_name_0"]`).Fill("Playwright Split A")).To(Succeed())
		Expect(page.Locator(`input[name="split_team_name_1"]`).Fill("")).To(Succeed())
		Expect(submitRegistrationIgnoringValidation(page)).To(Succeed())
		Expect(page.Locator(`text=Please provide a name for every split team.`).First().WaitFor()).To(Succeed())
		firstSplitName, err := page.Locator(`input[name="split_team_name_0"]`).InputValue()
		Expect(err).NotTo(HaveOccurred())
		Expect(firstSplitName).To(Equal("Playwright Split A"))
		secondSplitName, err := page.Locator(`input[name="split_team_name_1"]`).InputValue()
		Expect(err).NotTo(HaveOccurred())
		Expect(secondSplitName).To(Equal(""))
		Expect(page.Locator(`input[name="split_team_name_1"]`).Fill("Playwright Split B")).To(Succeed())
		Expect(page.Locator(`button:has-text("Yes, split and register")`).Click()).To(Succeed())
		Expect(page.Locator(`#registration-panel >> text=Registration successful. Your group was split into 2 teams (6, 5).`).First().WaitFor()).To(Succeed())

		registrations, err := listCollectionRecords(env.baseURL, env.systemAdminToken, "registrations")
		Expect(err).NotTo(HaveOccurred())
		splitA, err := recordIDByFieldValue(registrations, "team_name", "Playwright Split A")
		Expect(err).NotTo(HaveOccurred())
		splitB, err := recordIDByFieldValue(registrations, "team_name", "Playwright Split B")
		Expect(err).NotTo(HaveOccurred())
		Expect(splitA).NotTo(Equal(splitB))

		seatsAfterText, err := page.Locator(`#registration-panel p strong`).TextContent()
		Expect(err).NotTo(HaveOccurred())
		seatsAfter, err := strconv.Atoi(strings.TrimSpace(seatsAfterText))
		Expect(err).NotTo(HaveOccurred())
		Expect(seatsAfter).To(Equal(initialSeats - 11))
		Expect(page.Locator(`input[name="split_team_name_0"]`).Count()).To(Equal(0))
		Expect(page.Locator(`button:has-text("Yes, split and register")`).Count()).To(Equal(0))
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

		yesTeamName := fmt.Sprintf("Consent Yes Team %d", time.Now().UnixNano())
		yesEmail := fmt.Sprintf("consent-yes-%d@example.com", time.Now().UnixNano())
		_, err = page.Goto(env.baseURL+"/quiz?id="+quizID, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(`input[name="email"]`).Fill(yesEmail)).To(Succeed())
		Expect(page.Locator(`input[name="team_name"]`).Fill(yesTeamName)).To(Succeed())
		Expect(page.Locator(`input[name="team_size"]`).Fill("2")).To(Succeed())
		Expect(page.Locator(`button:has-text("Register")`).Click()).To(Succeed())
		Expect(page.Locator(`button:has-text("Yes, we can be merged")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Registration successful`).First().WaitFor()).To(Succeed())

		registrations, err := listCollectionRecords(env.baseURL, env.systemAdminToken, "registrations")
		Expect(err).NotTo(HaveOccurred())
		yesRecord, err := recordIDByFieldValue(registrations, "team_name", yesTeamName)
		Expect(err).NotTo(HaveOccurred())
		Expect(yesRecord).NotTo(BeEmpty())
		for _, record := range registrations {
			if teamName, ok := record["team_name"].(string); ok && teamName == yesTeamName {
				Expect(record["willing_to_merge"]).To(BeTrue())
				break
			}
		}
	})

	It("auto-closes full quizzes and quizzes within one hour of start", func() {
		now := time.Now()
		fullLocationID, err := createCollectionRecord(env.baseURL, env.systemAdminToken, "locations", map[string]any{
			"name":     fmt.Sprintf("Full Quiz Pub %d", now.UnixNano()),
			"maps_url": "https://maps.google.com/?q=FullQuizPub",
			"capacity": 2,
		})
		Expect(err).NotTo(HaveOccurred())
		fullQuizID, err := createCollectionRecord(env.baseURL, env.systemAdminToken, "quiz_dates", map[string]any{
			"location":     fullLocationID,
			"scheduled_at": now.Add(2 * time.Hour).UTC().Format(time.RFC3339),
			"is_open":      true,
		})
		Expect(err).NotTo(HaveOccurred())
		_, err = createCollectionRecord(env.baseURL, env.systemAdminToken, "registrations", map[string]any{
			"quiz":             fullQuizID,
			"email":            fmt.Sprintf("full-%d@example.com", now.UnixNano()),
			"team_name":        fmt.Sprintf("Full Team %d", now.UnixNano()),
			"team_size":        2,
			"willing_to_merge": true,
		})
		Expect(err).NotTo(HaveOccurred())

		timeLocationID, err := createCollectionRecord(env.baseURL, env.systemAdminToken, "locations", map[string]any{
			"name":     fmt.Sprintf("Soon Quiz Pub %d", now.UnixNano()),
			"maps_url": "https://maps.google.com/?q=SoonQuizPub",
			"capacity": 20,
		})
		Expect(err).NotTo(HaveOccurred())
		soonQuizID, err := createCollectionRecord(env.baseURL, env.systemAdminToken, "quiz_dates", map[string]any{
			"location":     timeLocationID,
			"scheduled_at": now.Add(30 * time.Minute).UTC().Format(time.RFC3339),
			"is_open":      true,
		})
		Expect(err).NotTo(HaveOccurred())

		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		_, err = page.Goto(env.baseURL, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(fmt.Sprintf(`a[href="/quiz?id=%s"]`, fullQuizID)).Count()).To(Equal(0))
		Expect(page.Locator(fmt.Sprintf(`a[href="/quiz?id=%s"]`, soonQuizID)).Count()).To(Equal(0))

		records, err := listCollectionRecords(env.baseURL, env.systemAdminToken, "quiz_dates")
		Expect(err).NotTo(HaveOccurred())
		for _, record := range records {
			if id, ok := record["id"].(string); ok && id == fullQuizID {
				Expect(record["is_open"]).To(BeFalse())
			}
			if id, ok := record["id"].(string); ok && id == soonQuizID {
				Expect(record["is_open"]).To(BeFalse())
			}
		}
	})

	It("shows the public empty state when no future quizzes exist", func() {
		records, err := listCollectionRecords(env.baseURL, env.systemAdminToken, "quiz_dates")
		Expect(err).NotTo(HaveOccurred())
		for _, record := range records {
			recordID, ok := record["id"].(string)
			Expect(ok).To(BeTrue())
			Expect(strings.TrimSpace(recordID)).NotTo(BeEmpty())
			Expect(deleteCollectionRecord(env.baseURL, env.systemAdminToken, "quiz_dates", recordID)).To(Succeed())
		}

		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		_, err = page.Goto(env.baseURL, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(`text=No upcoming quizzes yet.`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`a[href^="/quiz?id="]`).Count()).To(Equal(0))
	})

	It("shows the public setup error when quiz collections are missing", func() {
		port, err := freeLocalPort()
		Expect(err).NotTo(HaveOccurred())
		dataDir, err := os.MkdirTemp("", "pubquiz-playwright-setup-*")
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(os.RemoveAll(dataDir)).To(Succeed()) }()
		baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
		listenAddr := fmt.Sprintf("127.0.0.1:%d", port)

		var appLogs bytes.Buffer
		cmd := exec.Command("go", "run", "..", "serve", "--http", listenAddr, "--dir", dataDir, "--dev=false")
		cmd.Stdout = &appLogs
		cmd.Stderr = &appLogs
		Expect(cmd.Start()).To(Succeed())
		Expect(waitForHTTP(baseURL, 45*time.Second)).To(Succeed(), appLogs.String())
		defer func() {
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
				_, _ = cmd.Process.Wait()
			}
		}()

		setupAdminEmail := fmt.Sprintf("setup-admin-%d@example.com", time.Now().UnixNano())
		setupAdminPassword := "Password123!"
		Expect(createSystemAdminAccount(dataDir, setupAdminEmail, setupAdminPassword)).To(Succeed(), appLogs.String())
		setupAdminToken, err := loginSystemAdmin(baseURL, setupAdminEmail, setupAdminPassword)
		Expect(err).NotTo(HaveOccurred())

		Expect(deleteCollection(baseURL, setupAdminToken, "registrations")).To(Succeed())
		Expect(deleteCollection(baseURL, setupAdminToken, "quiz_dates")).To(Succeed())

		pageContext, err := env.browser.NewContext()
		Expect(err).NotTo(HaveOccurred())
		page, err := pageContext.NewPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(pageContext.Close()).To(Succeed()) }()

		_, err = page.Goto(baseURL, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(`text=The database collections are not ready yet.`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`text=Import pb_schema.json in the PocketBase Admin UI, then add locations and quiz dates.`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`a[href^="/quiz?id="]`).Count()).To(Equal(0))
	})
}
