package e2e_tests

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPlaywrightE2ESuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pubquiz Playwright E2E Suite")
}
