package main

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("admin.go helpers", func() {
	Describe("quiz admin helpers", func() {
		It("splits comma separated values", func() {
			Expect(splitCSVValues("a, b,\n c")).To(Equal([]string{"a", "b", "c"}))
		})

		It("formats stored date-times for datetime-local inputs", func() {
			formatted := formatDateTimeInputValue("2026-08-05T17:30:00Z")
			Expect(formatted).To(Equal(time.Date(2026, time.August, 5, 17, 30, 0, 0, time.UTC).Local().Format("2006-01-02T15:04")))
		})

		It("formats stored date-times for German labels", func() {
			formatted := formatGermanDateTimeLabel("2026-08-05T17:30:00Z")
			Expect(formatted).To(Equal(time.Date(2026, time.August, 5, 17, 30, 0, 0, time.UTC).Local().Format("02.01.2006 15:04")))
		})

		It("normalizes datetime-local input for storage", func() {
			normalized, err := normalizeDateTimeInput("2026-08-05T19:30")
			Expect(err).NotTo(HaveOccurred())

			expected := time.Date(2026, time.August, 5, 19, 30, 0, 0, time.Local).UTC().Format(time.RFC3339)
			Expect(normalized).To(Equal(expected))
		})

		It("rejects invalid date-time input", func() {
			_, err := normalizeDateTimeInput("not-a-date")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("admin formatting helpers", func() {
		It("formats admin values consistently", func() {
			Expect(formatAdminValue(nil)).To(Equal(""))
			Expect(formatAdminValue([]string{"a", "b"})).To(Equal("a, b"))
			Expect(formatAdminValue([]any{"a", 2, true})).To(Equal("a, 2, true"))
			Expect(formatAdminValue(42)).To(Equal("42"))
		})

		It("resolves relation labels for single values", func() {
			labels := map[string]string{"q1": "Quiz 1"}
			Expect(formatRelationValue("q1", labels, false)).To(Equal("Quiz 1"))
			Expect(formatRelationValue("unknown", labels, false)).To(Equal("unknown"))
			Expect(formatRelationValue("", labels, false)).To(Equal(""))
		})

		It("resolves relation labels for multiple values", func() {
			labels := map[string]string{"q1": "Quiz 1", "q2": "Quiz 2"}
			Expect(formatRelationValue([]string{"q1", "q2"}, labels, true)).To(Equal("Quiz 1, Quiz 2"))
		})

		It("normalizes string slices from various input shapes", func() {
			Expect(normalizeStringSlice([]string{" a ", "", "b"})).To(Equal([]string{"a", "b"}))
			Expect(normalizeStringSlice([]any{" a ", 2, ""})).To(Equal([]string{"a", "2"}))
			Expect(normalizeStringSlice("  x  ")).To(Equal([]string{"x"}))
			Expect(normalizeStringSlice(nil)).To(BeNil())
		})

		It("parses record date-time values", func() {
			when, ok := recordDateTime("2026-08-05T19:30:00Z")
			Expect(ok).To(BeTrue())
			Expect(when.IsZero()).To(BeFalse())

			_, ok = recordDateTime("invalid")
			Expect(ok).To(BeFalse())
		})
	})
})
