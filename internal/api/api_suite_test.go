package api

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAPISuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pubquiz API Suite")
}
