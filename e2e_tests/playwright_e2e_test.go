package e2e_tests

import (
	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("playwright e2e", Ordered, func() {
	env := &playwrightE2EEnv{}

	BeforeAll(env.setup)
	AfterAll(env.cleanup)

	Describe("UI", func() {
		defineUITests(env)
	})

	Describe("Functionality", func() {
		defineFunctionalityTests(env)
	})
})
