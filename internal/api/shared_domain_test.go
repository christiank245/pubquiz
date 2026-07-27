package api

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("shared_domain helpers", func() {
	Describe("NormalizeRegistrationIDs", func() {
		It("trims and de-duplicates while preserving order", func() {
			ids, err := NormalizeRegistrationIDs([]string{" a ", "b", "a", "", " b "})
			Expect(err).NotTo(HaveOccurred())
			Expect(ids).To(Equal([]string{"a", "b"}))
		})

		It("requires at least two unique ids", func() {
			_, err := NormalizeRegistrationIDs([]string{" one ", "one"})
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("ParseRegistrationIDsInput", func() {
		It("parses IDs from mixed separators", func() {
			ids, err := ParseRegistrationIDsInput("id-1, id-2\nid-3\tid-2")
			Expect(err).NotTo(HaveOccurred())
			Expect(ids).To(Equal([]string{"id-1", "id-2", "id-3"}))
		})
	})
})
