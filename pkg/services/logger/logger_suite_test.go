package logger_test

import (
	"github.com/dockerutil/shoutrrr/internal/testutils"
	unit "github.com/dockerutil/shoutrrr/pkg/services/logger"
	"github.com/dockerutil/shoutrrr/pkg/types"

	"github.com/onsi/gomega/gbytes"

	"log"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestLogger(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Logger Suite")
}

var _ = Describe("the logger service", func() {

	When("sending a notification", func() {

		It("should output the message to the log", func() {
			logbuf := gbytes.NewBuffer()
			service := &unit.Service{}
			_ = service.Initialize(testutils.URLMust(`logger://`), log.New(logbuf, "", 0))

			err := service.Send(`Failed - Requires Toaster Repair Level 10`, nil)
			Expect(err).NotTo(HaveOccurred())

			Eventually(logbuf).Should(gbytes.Say("Failed - Requires Toaster Repair Level 10"))
		})

		It("should not mutate the passed params", func() {
			service := &unit.Service{}
			_ = service.Initialize(testutils.URLMust(`logger://`), nil)
			params := types.Params{}
			err := service.Send(`Failed - Requires Toaster Repair Level 10`, &params)
			Expect(err).NotTo(HaveOccurred())

			Expect(params).To(BeEmpty())
		})

		When("when a template has been added", func() {
			It("should render template with params", func() {
				logbuf := gbytes.NewBuffer()
				service := &unit.Service{}
				_ = service.Initialize(testutils.URLMust(`logger://`), log.New(logbuf, "", 0))
				err := service.SetTemplateString(`message`, `{{.level}}: {{.message}}`)
				Expect(err).NotTo(HaveOccurred())

				params := types.Params{
					"level": "warning",
				}
				err = service.Send(`Requires Toaster Repair Level 10`, &params)
				Expect(err).NotTo(HaveOccurred())

				Eventually(logbuf).Should(gbytes.Say("warning: Requires Toaster Repair Level 10"))
			})
		})
	})
})
