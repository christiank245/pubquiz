package api

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("public_domain helpers", func() {
	Describe("SplitTeamSizes", func() {
		It("splits 11 into 6 and 5", func() {
			Expect(SplitTeamSizes(11, MaxTeamSize)).To(Equal([]int{6, 5}))
		})

		It("splits 20 into 10 and 10", func() {
			Expect(SplitTeamSizes(20, MaxTeamSize)).To(Equal([]int{10, 10}))
		})

		It("splits 21 into 7, 7, 7", func() {
			Expect(SplitTeamSizes(21, MaxTeamSize)).To(Equal([]int{7, 7, 7}))
		})

		It("never exceeds max team size and keeps total", func() {
			sizes := SplitTeamSizes(37, MaxTeamSize)

			total := 0
			for _, size := range sizes {
				Expect(size).To(BeNumerically("<=", MaxTeamSize))
				total += size
			}
			Expect(total).To(Equal(37))
		})
	})

	Describe("ShouldAutoCloseQuiz", func() {
		now := time.Date(2026, time.July, 24, 18, 0, 0, 0, time.UTC)

		It("closes when full", func() {
			Expect(ShouldAutoCloseQuiz(now.Add(4*time.Hour), 20, 20, now)).To(BeTrue())
		})

		It("closes within one hour", func() {
			Expect(ShouldAutoCloseQuiz(now.Add(45*time.Minute), 20, 5, now)).To(BeTrue())
		})

		It("closes exactly one hour before start", func() {
			Expect(ShouldAutoCloseQuiz(now.Add(1*time.Hour), 20, 5, now)).To(BeTrue())
		})

		It("stays open with more than one hour left and free seats", func() {
			Expect(ShouldAutoCloseQuiz(now.Add(61*time.Minute), 20, 5, now)).To(BeFalse())
		})
	})

	Describe("split confirmation helpers", func() {
		It("formats split sizes label", func() {
			Expect(JoinIntSizes([]int{7, 7, 7})).To(Equal("7, 7, 7"))
		})

		It("builds default split team names", func() {
			Expect(DefaultSplitTeamNames("Team", 3)).To(Equal([]string{
				"Team (1/3)", "Team (2/3)", "Team (3/3)",
			}))
		})

		It("detects empty values", func() {
			Expect(HasEmptyValues([]string{"ok", ""})).To(BeTrue())
			Expect(HasEmptyValues([]string{"ok", "still-ok"})).To(BeFalse())
		})
	})

	Describe("ParseMergeConsent", func() {
		It("accepts yes", func() {
			merge, err := ParseMergeConsent("yes")
			Expect(err).NotTo(HaveOccurred())
			Expect(merge).To(BeTrue())
		})

		It("accepts no", func() {
			merge, err := ParseMergeConsent("no")
			Expect(err).NotTo(HaveOccurred())
			Expect(merge).To(BeFalse())
		})

		It("accepts case and whitespace variants", func() {
			merge, err := ParseMergeConsent("  YeS ")
			Expect(err).NotTo(HaveOccurred())
			Expect(merge).To(BeTrue())
		})

		It("rejects invalid values", func() {
			_, err := ParseMergeConsent("maybe")
			Expect(err).To(HaveOccurred())
		})
	})

})
