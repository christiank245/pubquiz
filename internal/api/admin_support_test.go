package api

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("admin_support helpers", func() {
	Describe("quiz admin helpers", func() {
		It("splits comma separated values", func() {
			Expect(SplitCSVValues("a, b,\n c")).To(Equal([]string{"a", "b", "c"}))
		})

		It("formats stored date-times for datetime-local inputs", func() {
			formatted := FormatDateTimeInputValue("2026-08-05T17:30:00Z")
			Expect(formatted).To(Equal(time.Date(2026, time.August, 5, 17, 30, 0, 0, time.UTC).Local().Format("2006-01-02T15:04")))
		})

		It("formats stored date-times for German labels", func() {
			formatted := FormatGermanDateTimeLabel("2026-08-05T17:30:00Z")
			Expect(formatted).To(Equal(time.Date(2026, time.August, 5, 17, 30, 0, 0, time.UTC).Local().Format("02.01.2006 15:04")))
		})

		It("normalizes datetime-local input for storage", func() {
			normalized, err := NormalizeDateTimeInput("2026-08-05T19:30")
			Expect(err).NotTo(HaveOccurred())

			expected := time.Date(2026, time.August, 5, 19, 30, 0, 0, time.Local).UTC().Format(time.RFC3339)
			Expect(normalized).To(Equal(expected))
		})

		It("rejects invalid date-time input", func() {
			_, err := NormalizeDateTimeInput("not-a-date")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("admin formatting helpers", func() {
		It("formats admin values consistently", func() {
			Expect(FormatAdminValue(nil)).To(Equal(""))
			Expect(FormatAdminValue([]string{"a", "b"})).To(Equal("a, b"))
			Expect(FormatAdminValue([]any{"a", 2, true})).To(Equal("a, 2, true"))
			Expect(FormatAdminValue(42)).To(Equal("42"))
		})

		It("resolves relation labels for single values", func() {
			labels := map[string]string{"q1": "Quiz 1"}
			Expect(FormatRelationValue("q1", labels, false)).To(Equal("Quiz 1"))
			Expect(FormatRelationValue("unknown", labels, false)).To(Equal("unknown"))
			Expect(FormatRelationValue("", labels, false)).To(Equal(""))
		})

		It("resolves relation labels for multiple values", func() {
			labels := map[string]string{"q1": "Quiz 1", "q2": "Quiz 2"}
			Expect(FormatRelationValue([]string{"q1", "q2"}, labels, true)).To(Equal("Quiz 1, Quiz 2"))
		})

		It("normalizes string slices from various input shapes", func() {
			Expect(NormalizeStringSlice([]string{" a ", "", "b"})).To(Equal([]string{"a", "b"}))
			Expect(NormalizeStringSlice([]any{" a ", 2, ""})).To(Equal([]string{"a", "2"}))
			Expect(NormalizeStringSlice("  x  ")).To(Equal([]string{"x"}))
			Expect(NormalizeStringSlice(nil)).To(BeNil())
		})
	})
})
