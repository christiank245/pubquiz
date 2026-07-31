package e2e_tests

import (
	"fmt"
	"time"

	"github.com/mxschmitt/playwright-go"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func defineAdminRegistrationTests(env *playwrightE2EEnv) {
	It("supports registration merge and delete flows", func() {
		page, cleanup, err := env.newPage()
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(cleanup()).To(Succeed()) }()

		quizScheduledAt := time.Now().Add(2 * time.Hour).UTC()
		locationName := fmt.Sprintf("Merge Pub %d", time.Now().UnixNano())
		locationID, err := createCollectionRecord(env.baseURL, env.systemAdminToken, "locations", map[string]any{
			"name":     locationName,
			"maps_url": "https://maps.google.com/?q=MergePub",
			"capacity": 40,
		})
		Expect(err).NotTo(HaveOccurred())
		quizID, err := createCollectionRecord(env.baseURL, env.systemAdminToken, "quiz_dates", map[string]any{
			"location":     locationID,
			"scheduled_at": quizScheduledAt.Format(time.RFC3339),
			"is_open":      true,
		})
		Expect(err).NotTo(HaveOccurred())
		quizLabel := quizScheduledAt.Local().Format("02.01.2006 15:04") + " - " + locationName

		teamA := fmt.Sprintf("Merge A %d", time.Now().UnixNano())
		teamB := fmt.Sprintf("Merge B %d", time.Now().UnixNano())
		teamEdit := fmt.Sprintf("Edit Team %d", time.Now().UnixNano())
		otherLocationID, err := createCollectionRecord(env.baseURL, env.systemAdminToken, "locations", map[string]any{
			"name":     fmt.Sprintf("Other Merge Pub %d", time.Now().UnixNano()),
			"maps_url": "https://maps.google.com/?q=OtherMergePub",
			"capacity": 40,
		})
		Expect(err).NotTo(HaveOccurred())
		otherQuizScheduledAt := time.Now().Add(3 * time.Hour).UTC()
		otherQuizID, err := createCollectionRecord(env.baseURL, env.systemAdminToken, "quiz_dates", map[string]any{
			"location":     otherLocationID,
			"scheduled_at": otherQuizScheduledAt.Format(time.RFC3339),
			"is_open":      true,
		})
		Expect(err).NotTo(HaveOccurred())
		otherTeam := fmt.Sprintf("Other Quiz Team %d", time.Now().UnixNano())

		teamAEmail := fmt.Sprintf("a-%d@example.com", time.Now().UnixNano())
		_, err = createCollectionRecord(env.baseURL, env.systemAdminToken, "registrations", map[string]any{
			"quiz":             quizID,
			"email":            teamAEmail,
			"team_name":        teamA,
			"team_size":        2,
			"willing_to_merge": true,
		})
		Expect(err).NotTo(HaveOccurred())
		teamBEmail := fmt.Sprintf("b-%d@example.com", time.Now().UnixNano())
		_, err = createCollectionRecord(env.baseURL, env.systemAdminToken, "registrations", map[string]any{
			"quiz":             quizID,
			"email":            teamBEmail,
			"team_name":        teamB,
			"team_size":        3,
			"willing_to_merge": true,
		})
		Expect(err).NotTo(HaveOccurred())
		_, err = createCollectionRecord(env.baseURL, env.systemAdminToken, "registrations", map[string]any{
			"quiz":             quizID,
			"email":            fmt.Sprintf("edit-%d@example.com", time.Now().UnixNano()),
			"team_name":        teamEdit,
			"team_size":        4,
			"willing_to_merge": false,
		})
		Expect(err).NotTo(HaveOccurred())
		_, err = createCollectionRecord(env.baseURL, env.systemAdminToken, "registrations", map[string]any{
			"quiz":             otherQuizID,
			"email":            fmt.Sprintf("other-%d@example.com", time.Now().UnixNano()),
			"team_name":        otherTeam,
			"team_size":        2,
			"willing_to_merge": true,
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(loginQuizAdmin(page, env.baseURL, env.quizAdminEmail, env.quizAdminPassword)).To(Succeed())
		_, err = page.Goto(env.baseURL+"/quiz-admin/collections/registrations", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s")`, quizLabel)).First().WaitFor()).To(Succeed())
		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s")`, teamA)).First().WaitFor()).To(Succeed())
		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s")`, teamB)).First().WaitFor()).To(Succeed())
		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s")`, teamEdit)).First().WaitFor()).To(Succeed())
		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s")`, otherTeam)).First().WaitFor()).To(Succeed())

		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s") a:has-text("%s")`, teamEdit, teamEdit)).First().Click()).To(Succeed())
		Expect(page.Locator(`#admin-edit-modal:not(.hidden)`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`input[name="field_team_name"]`).Fill(teamEdit + " Updated")).To(Succeed())
		Expect(page.Locator(`button:has-text("Save changes")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Entry updated.`).First().WaitFor()).To(Succeed())
		teamEdit = teamEdit + " Updated"

		// invalid: not mergeable
		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s") input[name="delete_ids"]`, teamA)).First().Check()).To(Succeed())
		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s") input[name="delete_ids"]`, teamEdit)).First().Check()).To(Succeed())
		disabled, err := page.Locator(`#merge-entries-button`).IsDisabled()
		Expect(err).NotTo(HaveOccurred())
		Expect(disabled).To(BeTrue())

		_, err = page.Goto(env.baseURL+"/quiz-admin/collections/registrations", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s") input[name="delete_ids"]`, teamA)).First().Check()).To(Succeed())
		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s") input[name="delete_ids"]`, otherTeam)).First().Check()).To(Succeed())
		disabled, err = page.Locator(`#merge-entries-button`).IsDisabled()
		Expect(err).NotTo(HaveOccurred())
		Expect(disabled).To(BeTrue())

		_, err = page.Goto(env.baseURL+"/quiz-admin/collections/registrations", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s") input[name="delete_ids"]`, teamA)).First().Check()).To(Succeed())
		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s") input[name="delete_ids"]`, teamB)).First().Check()).To(Succeed())
		Expect(page.Locator(`#merge-entries-button:not([disabled])`).First().WaitFor()).To(Succeed())
		disabled, err = page.Locator(`#merge-entries-button`).IsDisabled()
		Expect(err).NotTo(HaveOccurred())
		Expect(disabled).To(BeFalse())
		Expect(page.Locator(`#merge-entries-button`).Click()).To(Succeed())
		Expect(page.Locator(`h3:has-text("Merge entries")`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`text=2 entries from ` + quizLabel + ` will be merged.`).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`text=` + teamA).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`text=` + teamB).First().WaitFor()).To(Succeed())
		Expect(page.Locator(`#merge-entries-modal button:has-text("Merge selected teams")`).Click()).To(Succeed())
		Expect(page.Locator(`text=Entries merged successfully into`).First().WaitFor()).To(Succeed())

		registrations, err := listCollectionRecords(env.baseURL, env.systemAdminToken, "registrations")
		Expect(err).NotTo(HaveOccurred())
		var mergedRecord map[string]any
		for _, record := range registrations {
			if fmt.Sprint(record["quiz"]) == quizID && fmt.Sprint(record["team_size"]) == "5" {
				mergedRecord = record
				break
			}
		}
		Expect(mergedRecord).NotTo(BeNil())
		mergedTeamName := fmt.Sprint(mergedRecord["team_name"])
		Expect(mergedTeamName).To(ContainSubstring("Merge A"))
		Expect(mergedTeamName).To(ContainSubstring("Merge B"))
		mergedEmail := fmt.Sprint(mergedRecord["email"])
		Expect(mergedEmail == teamAEmail || mergedEmail == teamBEmail).To(BeTrue())
		_, err = recordIDByFieldValue(registrations, "team_name", teamA)
		Expect(err).To(HaveOccurred())
		_, err = recordIDByFieldValue(registrations, "team_name", teamB)
		Expect(err).To(HaveOccurred())

		_, err = page.Goto(env.baseURL+"/quiz-admin/collections/registrations", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s") input[name="delete_ids"]`, teamEdit)).First().Check()).To(Succeed())
		Expect(page.Locator(fmt.Sprintf(`tr:has-text("%s") input[name="delete_ids"]`, otherTeam)).First().Check()).To(Succeed())
		Expect(page.Locator(`button:has-text("Delete selected")`).Click()).To(Succeed())
		Expect(page.Locator(`#admin-delete-modal button:has-text("Delete")`).First().Click()).To(Succeed())
		Expect(page.Locator(`text=Deleted 2 entrie(s).`).First().WaitFor()).To(Succeed())
	})
}
