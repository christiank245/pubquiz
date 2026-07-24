package main

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("registration and quiz helpers", func() {
	Describe("splitTeamSizes", func() {
		It("splits 11 into 6 and 5", func() {
			Expect(splitTeamSizes(11, maxTeamSize)).To(Equal([]int{6, 5}))
		})

		It("splits 20 into 10 and 10", func() {
			Expect(splitTeamSizes(20, maxTeamSize)).To(Equal([]int{10, 10}))
		})

		It("splits 21 into 7, 7, 7", func() {
			Expect(splitTeamSizes(21, maxTeamSize)).To(Equal([]int{7, 7, 7}))
		})

		It("never exceeds max team size and keeps total", func() {
			sizes := splitTeamSizes(37, maxTeamSize)

			total := 0
			for _, size := range sizes {
				Expect(size).To(BeNumerically("<=", maxTeamSize))
				total += size
			}
			Expect(total).To(Equal(37))
		})
	})

	Describe("shouldAutoCloseQuiz", func() {
		now := time.Date(2026, time.July, 24, 18, 0, 0, 0, time.UTC)

		It("closes when full", func() {
			Expect(shouldAutoCloseQuiz(now.Add(4*time.Hour), 20, 20, now)).To(BeTrue())
		})

		It("closes within one hour", func() {
			Expect(shouldAutoCloseQuiz(now.Add(45*time.Minute), 20, 5, now)).To(BeTrue())
		})

		It("closes exactly one hour before start", func() {
			Expect(shouldAutoCloseQuiz(now.Add(1*time.Hour), 20, 5, now)).To(BeTrue())
		})

		It("stays open with more than one hour left and free seats", func() {
			Expect(shouldAutoCloseQuiz(now.Add(61*time.Minute), 20, 5, now)).To(BeFalse())
		})
	})

	Describe("split confirmation helpers", func() {
		It("formats split sizes label", func() {
			Expect(joinIntSizes([]int{7, 7, 7})).To(Equal("7, 7, 7"))
		})

		It("builds default split team names", func() {
			Expect(defaultSplitTeamNames("Team", 3)).To(Equal([]string{
				"Team (1/3)", "Team (2/3)", "Team (3/3)",
			}))
		})

		It("detects empty values", func() {
			Expect(hasEmptyValues([]string{"ok", ""})).To(BeTrue())
			Expect(hasEmptyValues([]string{"ok", "still-ok"})).To(BeFalse())
		})
	})

	Describe("parseMergeConsent", func() {
		It("accepts yes", func() {
			merge, err := parseMergeConsent("yes")
			Expect(err).NotTo(HaveOccurred())
			Expect(merge).To(BeTrue())
		})

		It("accepts no", func() {
			merge, err := parseMergeConsent("no")
			Expect(err).NotTo(HaveOccurred())
			Expect(merge).To(BeFalse())
		})

		It("accepts case and whitespace variants", func() {
			merge, err := parseMergeConsent("  YeS ")
			Expect(err).NotTo(HaveOccurred())
			Expect(merge).To(BeTrue())
		})

		It("rejects invalid values", func() {
			_, err := parseMergeConsent("maybe")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("normalizeRegistrationIDs", func() {
		It("trims and de-duplicates while preserving order", func() {
			ids, err := normalizeRegistrationIDs([]string{" a ", "b", "a", "", " b "})
			Expect(err).NotTo(HaveOccurred())
			Expect(ids).To(Equal([]string{"a", "b"}))
		})

		It("requires at least two unique ids", func() {
			_, err := normalizeRegistrationIDs([]string{" one ", "one"})
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("parseRegistrationIDsInput", func() {
		It("parses IDs from mixed separators", func() {
			ids, err := parseRegistrationIDsInput("id-1, id-2\nid-3\tid-2")
			Expect(err).NotTo(HaveOccurred())
			Expect(ids).To(Equal([]string{"id-1", "id-2", "id-3"}))
		})
	})

	Describe("parseOptionalMergeBool", func() {
		It("returns nil for auto", func() {
			value, err := parseOptionalMergeBool("auto")
			Expect(err).NotTo(HaveOccurred())
			Expect(value).To(BeNil())
		})

		It("parses true and false", func() {
			trueValue, err := parseOptionalMergeBool("true")
			Expect(err).NotTo(HaveOccurred())
			Expect(trueValue).NotTo(BeNil())
			Expect(*trueValue).To(BeTrue())

			falseValue, err := parseOptionalMergeBool("false")
			Expect(err).NotTo(HaveOccurred())
			Expect(falseValue).NotTo(BeNil())
			Expect(*falseValue).To(BeFalse())
		})

		It("rejects invalid values", func() {
			_, err := parseOptionalMergeBool("yes")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("sanitizeAdminRedirect", func() {
		It("keeps safe relative paths", func() {
			Expect(sanitizeAdminRedirect("/admin/registrations/merge")).To(Equal("/admin/registrations/merge"))
		})

		It("falls back on empty or unsafe values", func() {
			Expect(sanitizeAdminRedirect("")).To(Equal("/admin/registrations/merge"))
			Expect(sanitizeAdminRedirect("https://example.com")).To(Equal("/admin/registrations/merge"))
			Expect(sanitizeAdminRedirect("//evil.example.com")).To(Equal("/admin/registrations/merge"))
		})
	})

	Describe("quizIDExists", func() {
		quizzes := []QuizCard{
			{ID: "quiz-a"},
			{ID: "quiz-b"},
		}

		It("returns true for existing quiz ids", func() {
			Expect(quizIDExists(quizzes, "quiz-a")).To(BeTrue())
		})

		It("returns false for unknown quiz ids", func() {
			Expect(quizIDExists(quizzes, "quiz-c")).To(BeFalse())
		})
	})
})
