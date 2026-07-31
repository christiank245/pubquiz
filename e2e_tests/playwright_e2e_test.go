package e2e_tests

import (
	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("playwright e2e", Ordered, func() {
	env := &playwrightE2EEnv{}

	BeforeAll(env.setup)
	AfterAll(env.cleanup)

	Describe("Public", func() {
		definePublicTests(env)
	})

	Describe("Admin general", func() {
		defineAdminGeneralTests(env)
	})

	Describe("Admin locations", func() {
		defineAdminLocationTests(env)
	})

	Describe("Admin quiz dates", func() {
		defineAdminQuizDateTests(env)
	})

	Describe("Admin registrations", func() {
		defineAdminRegistrationTests(env)
	})
})
