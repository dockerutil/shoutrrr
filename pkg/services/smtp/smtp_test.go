package smtp

import (
	"log"
	"net/smtp"
	"net/url"
	"os"
	"reflect"
	"testing"
	"unsafe"

	"github.com/dockerutil/shoutrrr/internal/failures"
	"github.com/dockerutil/shoutrrr/internal/testutils"
	"github.com/dockerutil/shoutrrr/pkg/format"
	"github.com/dockerutil/shoutrrr/pkg/services/standard"
	"github.com/dockerutil/shoutrrr/pkg/types"

	gt "github.com/onsi/gomega/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var tt *testing.T

func TestSMTP(t *testing.T) {
	RegisterFailHandler(Fail)
	tt = t
	RunSpecs(t, "Shoutrrr SMTP Suite")
}

var (
	service    *Service
	envSMTPURL string
	logger     *log.Logger
	_          = BeforeSuite(func() {

		envSMTPURL = os.Getenv("SHOUTRRR_SMTP_URL")
		logger = testutils.TestLogger()
	})
	urlWithAllProps = "smtp://user:password@example.com:2225/?auth=None&clienthost=testhost&encryption=ExplicitTLS&fromaddress=sender%40example.com&fromname=Sender&subject=Subject&toaddresses=rec1%40example.com%2Crec2%40example.com&usehtml=Yes&usestarttls=No"
)

var _ = Describe("the SMTP service", func() {

	BeforeEach(func() {
		service = &Service{}

	})
	When("parsing the configuration URL", func() {
		It("should be identical after de-/serialization", func() {
			url := testutils.URLMust(urlWithAllProps)
			config := &Config{}
			pkr := format.NewPropKeyResolver(config)
			err := config.setURL(&pkr, url)
			Expect(err).NotTo(HaveOccurred(), "verifying")

			outputURL := config.GetURL()
			GinkgoT().Logf("\n\n%s\n%s\n\n-", outputURL, urlWithAllProps)

			Expect(outputURL.String()).To(Equal(urlWithAllProps))

		})
		When("resolving client host", func() {
			When("clienthost is set to auto", func() {
				It("should return the os hostname", func() {
					hostname, err := os.Hostname()
					Expect(err).ToNot(HaveOccurred())
					Expect(service.resolveClientHost(&Config{ClientHost: "auto"})).To(Equal(hostname))
				})
			})
			When("clienthost is set to a custom value", func() {
				It("should return that value", func() {
					Expect(service.resolveClientHost(&Config{ClientHost: "computah"})).To(Equal("computah"))
				})
			})
		})
		When("fromAddress is missing", func() {
			It("should return an error", func() {
				testURL := testutils.URLMust("smtp://user:password@example.com:2225/?toAddresses=rec1@example.com,rec2@example.com")
				Expect((&Config{}).SetURL(testURL)).ToNot(Succeed())
			})
		})
		When("toAddresses are missing", func() {
			It("should return an error", func() {
				testURL := testutils.URLMust("smtp://user:password@example.com:2225/?fromAddress=sender@example.com")
				Expect((&Config{}).SetURL(testURL)).NotTo(Succeed())
			})

		})
	})
	Context("basic service API methods", func() {
		var config *Config
		BeforeEach(func() {
			config = &Config{}
		})
		It("should not allow getting invalid query values", func() {
			testutils.TestConfigGetInvalidQueryValue(config)
		})
		It("should not allow setting invalid query values", func() {
			testutils.TestConfigSetInvalidQueryValue(config, "smtp://example.com/?fromAddress=s@example.com&toAddresses=r@example.com&foo=bar")
		})

		It("should have the expected number of fields and enums", func() {
			testutils.TestConfigGetEnumsCount(config, 2)
			testutils.TestConfigGetFieldsCount(config, 13)
		})
	})
	When("cloning a config", func() {
		It("should be identical to the original", func() {
			config := &Config{}
			Expect(config.SetURL(testutils.URLMust(urlWithAllProps))).To(Succeed())

			Expect(config.Clone()).To(Equal(*config))

		})
	})
	When("sending a message", func() {
		When("the service is not configured correctly", func() {
			It("should fail to send messages", func() {
				service := Service{config: &Config{}}
				Expect(service.Send("test message", nil)).To(matchFailure(FailGetSMTPClient))

				service.config.Encryption = EncMethods.ImplicitTLS
				Expect(service.Send("test message", nil)).To(matchFailure(FailGetSMTPClient))
			})
		})
		When("an invalid param is passed", func() {
			It("should fail to send messages", func() {
				service := Service{config: &Config{}}
				Expect(service.Send("test message", &types.Params{"invalid": "value"})).To(matchFailure(FailApplySendParams))
			})
		})
	})

	When("the underlying stream stops working", func() {
		var service Service
		var message string
		BeforeEach(func() {
			service = Service{}
			message = ""
		})
		It("should fail when writing multipart plain header", func() {
			writer := testutils.CreateFailWriter(1)
			err := service.writeMultipartMessage(writer, message)
			Expect(err).To(HaveOccurred())
			Expect(err.ID()).To(Equal(FailPlainHeader))
		})

		It("should fail when writing multipart plain message", func() {
			writer := testutils.CreateFailWriter(2)
			err := service.writeMultipartMessage(writer, message)
			Expect(err).To(HaveOccurred())
			Expect(err.ID()).To(Equal(FailMessageRaw))
		})

		It("should fail when writing multipart HTML header", func() {
			writer := testutils.CreateFailWriter(4)
			err := service.writeMultipartMessage(writer, message)
			Expect(err).To(HaveOccurred())
			Expect(err.ID()).To(Equal(FailHTMLHeader))
		})

		It("should fail when writing multipart HTML message", func() {
			writer := testutils.CreateFailWriter(5)
			err := service.writeMultipartMessage(writer, message)
			Expect(err).To(HaveOccurred())
			Expect(err.ID()).To(Equal(FailMessageRaw))
		})

		It("should fail when writing multipart end header", func() {
			writer := testutils.CreateFailWriter(6)
			err := service.writeMultipartMessage(writer, message)
			Expect(err).To(HaveOccurred())
			Expect(err.ID()).To(Equal(FailMultiEndHeader))
		})

		It("should fail when writing message template", func() {
			writer := testutils.CreateFailWriter(0)
			e := service.SetTemplateString("dummy", "dummy template content")
			Expect(e).ToNot(HaveOccurred())

			err := service.writeMessagePart(writer, message, "dummy")
			Expect(err).To(HaveOccurred())
			Expect(err.ID()).To(Equal(FailMessageTemplate))
		})

	})

	When("running E2E tests", func() {

		It("should work without errors", func() {
			if envSMTPURL == "" {
				Skip("environment not set up for E2E testing")
				return
			}

			serviceURL, err := url.Parse(envSMTPURL)
			Expect(err).NotTo(HaveOccurred())

			err = service.Initialize(serviceURL, logger)
			Expect(err).NotTo(HaveOccurred())

			err = service.Send("this is an integration test", nil)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("running integration tests", func() {

		When("given a typical usage case configuration URL", func() {

			It("should send notifications without any errors", func() {
				testURL := "smtp://user:password@example.com:2225/?useStartTLS=no&fromAddress=sender@example.com&toAddresses=rec1@example.com,rec2@example.com&useHTML=yes"
				err := testIntegration(testURL, []string{
					"250-mx.google.com at your service",
					"250-SIZE 35651584",
					"250-AUTH LOGIN PLAIN",
					"250 8BITMIME",
					"235 Accepted",
					"250 Sender OK",
					"250 Receiver OK",
					"354 Go ahead",
					"250 Data OK",
					"250 Sender OK",
					"250 Receiver OK",
					"354 Go ahead",
					"250 Data OK",
					"221 OK",
				}, "<pre>{{ .message }}</pre>", "{{ .message }}")
				if msg, test := standard.IsTestSetupFailure(err); test {
					Skip(msg)
					return
				}
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("given e-mail addresses with pluses in the configuration URL", func() {

			It("should send notifications without any errors", func() {
				testURL := "smtp://user:password@example.com:2225/?useStartTLS=no&fromAddress=sender+tag@example.com&toAddresses=rec1+tag@example.com,rec2@example.com&useHTML=yes"
				err := testIntegration(
					testURL,
					[]string{
						"250-mx.google.com at your service",
						"250-SIZE 35651584",
						"250-AUTH LOGIN PLAIN",
						"250 8BITMIME",
						"235 Accepted",
						"250 Sender OK",
						"250 Receiver OK",
						"354 Go ahead",
						"250 Data OK",
						"250 Sender OK",
						"250 Receiver OK",
						"354 Go ahead",
						"250 Data OK",
						"221 OK",
					},
					// Message templates:
					"<pre>{{ .message }}</pre>", "{{ .message }}",
					// Expected to be sent from client
					"RCPT TO:<rec1+tag@example.com>",
					"To: rec1+tag@example.com",
					"From:  <sender+tag@example.com>")
				if msg, test := standard.IsTestSetupFailure(err); test {
					Skip(msg)
					return
				}
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("given a configuration URL with authentication disabled", func() {

			It("should send notifications without any errors", func() {
				testURL := "smtp://example.com:2225/?useStartTLS=no&fromAddress=sender@example.com&toAddresses=rec1@example.com&useHTML=no"
				err := testIntegration(testURL, []string{
					"250-mx.google.com at your service",
					"250-SIZE 35651584",
					"250-AUTH LOGIN PLAIN",
					"250 8BITMIME",
					"250 Sender OK",
					"250 Receiver OK",
					"354 Go ahead",
					"250 Data OK",
					"221 OK",
				}, "", "")
				if msg, test := standard.IsTestSetupFailure(err); test {
					Skip(msg)
					return
				}
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("given a configuration URL with StartTLS but it is not supported", func() {

			It("should send notifications without any errors", func() {
				testURL := "smtp://example.com:2225/?useStartTLS=yes&fromAddress=sender@example.com&toAddresses=rec1@example.com&useHTML=no"
				err := testIntegration(testURL, []string{
					"250-mx.google.com at your service",
					"250-SIZE 35651584",
					"250-AUTH LOGIN PLAIN",
					"250 8BITMIME",
					"250 Sender OK",
					"250 Receiver OK",
					"354 Go ahead",
					"250 Data OK",
					"221 OK",
				}, "", "")
				if msg, test := standard.IsTestSetupFailure(err); test {
					Skip(msg)
					return
				}
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("server communication fails", func() {

			It("should fail when initial handshake is not accepted", func() {
				testURL := "smtp://example.com:2225/?useStartTLS=yes&auth=none&fromAddress=sender@example.com&toAddresses=rec1@example.com&useHTML=no&clienthost=spammer"
				err := testIntegration(testURL, []string{
					"421 4.7.0 Try again later, closing connection. (EHLO) r20-20020a50d694000000b004588af8956dsm771862edi.9 - gsmtp",
				}, "", "")
				if msg, test := standard.IsTestSetupFailure(err); test {
					Skip(msg)
					return
				}
				Expect(err).To(MatchError(fail(FailHandshake, nil)))
			})

			It("should fail when not being able to enable StartTLS", func() {
				testURL := "smtp://example.com:2225/?useStartTLS=yes&auth=none&fromAddress=sender@example.com&toAddresses=rec1@example.com&useHTML=no"
				err := testIntegration(testURL, []string{
					"250-mx.google.com at your service",
					"250-SIZE 35651584",
					"250-STARTTLS",
					"250-AUTH LOGIN PLAIN",
					"250 8BITMIME",
					"502 That's too hard",
				}, "", "")
				if msg, test := standard.IsTestSetupFailure(err); test {
					Skip(msg)
					return
				}
				Expect(err).To(matchFailure(FailEnableStartTLS))
			})

			It("should fail when authentication type is invalid", func() {
				testURL := "smtp://example.com:2225/?useStartTLS=no&auth=bad&fromAddress=sender@example.com&toAddresses=rec1@example.com&useHTML=no"
				err := testIntegration(testURL, []string{}, "", "")
				if msg, test := standard.IsTestSetupFailure(err); test {
					Skip(msg)
					return
				}
				Expect(err).To(matchFailure(standard.FailServiceInit))
			})

			It("should fail when not being able to use authentication type", func() {
				testURL := "smtp://example.com:2225/?useStartTLS=no&auth=crammd5&fromAddress=sender@example.com&toAddresses=rec1@example.com&useHTML=no"
				err := testIntegration(testURL, []string{
					"250-mx.google.com at your service",
					"250-SIZE 35651584",
					"250-AUTH LOGIN PLAIN",
					"250 8BITMIME",
					"504 Liar",
				}, "", "")
				if msg, test := standard.IsTestSetupFailure(err); test {
					Skip(msg)
					return
				}
				Expect(err).To(matchFailure(FailAuthenticating))
			})

			It("should fail when not being able to send to recipient", func() {
				testURL := "smtp://example.com:2225/?useStartTLS=no&auth=none&fromAddress=sender@example.com&toAddresses=rec1@example.com&useHTML=no"
				err := testIntegration(testURL, []string{
					"250-mx.google.com at your service",
					"250-SIZE 35651584",
					"250-AUTH LOGIN PLAIN",
					"250 8BITMIME",
					"551 I don't know you",
				}, "", "")
				if msg, test := standard.IsTestSetupFailure(err); test {
					Skip(msg)
					return
				}
				Expect(err).To(matchFailure(FailSendRecipient))
			})

			It("should fail when the recipient is not accepted", func() {
				testURL := "smtp://example.com:2225/?useStartTLS=no&auth=none&fromAddress=sender@example.com&toAddresses=rec1@example.com&useHTML=no"
				err := testSendRecipient(testURL, []string{
					"250 mx.google.com at your service",
					"250 Sender OK",
					"553 She doesn't want to be disturbed",
				})
				if msg, test := standard.IsTestSetupFailure(err); test {
					Skip(msg)
					return
				}
				Expect(err).To(matchFailure(FailSetRecipient))
			})

			It("should fail when the server does not accept the data stream", func() {
				testURL := "smtp://example.com:2225/?useStartTLS=no&auth=none&fromAddress=sender@example.com&toAddresses=rec1@example.com&useHTML=no"
				err := testSendRecipient(testURL, []string{
					"250 mx.google.com at your service",
					"250 Sender OK",
					"250 Receiver OK",
					"554 Nah I'm fine thanks",
				})
				if msg, test := standard.IsTestSetupFailure(err); test {
					Skip(msg)
					return
				}
				Expect(err).To(matchFailure(FailOpenDataStream))
			})

			It("should fail when the server does not accept the data stream content", func() {
				testURL := "smtp://example.com:2225/?useStartTLS=no&auth=none&fromAddress=sender@example.com&toAddresses=rec1@example.com&useHTML=no"
				err := testSendRecipient(testURL, []string{
					"250 mx.google.com at your service",
					"250 Sender OK",
					"250 Receiver OK",
					"354 Go ahead",
					"554 Such garbage!",
				})
				if msg, test := standard.IsTestSetupFailure(err); test {
					Skip(msg)
					return
				}
				Expect(err).To(matchFailure(FailCloseDataStream))
			})

			It("should fail when the server does not close the connection gracefully", func() {
				testURL := "smtp://example.com:2225/?useStartTLS=no&auth=none&fromAddress=sender@example.com&toAddresses=rec1@example.com&useHTML=no"
				err := testIntegration(testURL, []string{
					"250-mx.google.com at your service",
					"250-SIZE 35651584",
					"250-AUTH LOGIN PLAIN",
					"250 8BITMIME",
					"250 Sender OK",
					"250 Receiver OK",
					"354 Go ahead",
					"250 Data OK",
					"502 You can't quit, you're fired!",
				}, "", "")
				if msg, test := standard.IsTestSetupFailure(err); test {
					Skip(msg)
					return
				}
				Expect(err).To(matchFailure(FailClosingSession))
			})

		})
	})
	When("writing headers and the output stream is closed", func() {
		When("it's closed during header content", func() {
			It("should fail with correct error", func() {
				fw := testutils.CreateFailWriter(0)
				Expect(writeHeaders(fw, map[string]string{"key": "value"})).To(matchFailure(FailWriteHeaders))
			})
		})
		When("it's closed after header content", func() {
			It("should fail with correct error", func() {
				fw := testutils.CreateFailWriter(1)
				Expect(writeHeaders(fw, map[string]string{"key": "value"})).To(matchFailure(FailWriteHeaders))
			})
		})
	})
})

func testSendRecipient(testURL string, responses []string) failures.Failure {
	serviceURL, err := url.Parse(testURL)
	if err != nil {
		return standard.Failure(standard.FailParseURL, err)
	}

	err = service.Initialize(serviceURL, logger)
	if err != nil {
		return failures.Wrap("error parsing URL", standard.FailTestSetup, err)
	}

	if err := service.SetTemplateString("plain", "{{.message}}"); err != nil {
		return failures.Wrap("error setting plain template", standard.FailTestSetup, err)
	}

	textCon, tcfaker := testutils.CreateTextConFaker(responses, "\r\n")

	client := &smtp.Client{
		Text: textCon,
	}

	fakeTLSEnabled(client, serviceURL.Hostname())

	config := &Config{}
	message := "message body"

	ferr := service.sendToRecipient(client, "r@example.com", config, message)

	logger.Printf("\n%s", tcfaker.GetConversation(false))
	if ferr != nil {
		return ferr
	}

	return nil
}

func testIntegration(testURL string, responses []string, htmlTemplate string, plainTemplate string, expectRec ...string) failures.Failure {

	serviceURL, err := url.Parse(testURL)
	if err != nil {
		return standard.Failure(standard.FailParseURL, err)
	}

	if err = service.Initialize(serviceURL, logger); err != nil {
		return standard.Failure(standard.FailServiceInit, err)
	}

	if htmlTemplate != "" {
		if err := service.SetTemplateString("HTML", htmlTemplate); err != nil {
			return failures.Wrap("error setting HTML template", standard.FailTestSetup, err)
		}
	}

	if plainTemplate != "" {
		if err := service.SetTemplateString("plain", plainTemplate); err != nil {
			return failures.Wrap("error setting plain template", standard.FailTestSetup, err)
		}
	}

	textCon, tcfaker := testutils.CreateTextConFaker(responses, "\r\n")

	client := &smtp.Client{
		Text: textCon,
	}

	fakeTLSEnabled(client, serviceURL.Hostname())

	ferr := service.doSend(client, "Test message", service.config)

	recieved := tcfaker.GetClientSentences()
	for _, expected := range expectRec {
		Expect(recieved).To(ContainElement(expected))
	}

	logger.Printf("\n%s", tcfaker.GetConversation(false))
	if ferr != nil {
		return ferr
	}

	return nil
}

// fakeTLSEnabled tricks a given client into believing that TLS is enabled even though it's not
// this is needed because the SMTP library won't allow plain authentication without TLS being turned on.
// having it turned on would of course mean that we cannot test the communication since it will be encrypted.
func fakeTLSEnabled(client *smtp.Client, hostname string) {

	// set the "tls" flag on the client which indicates that TLS encryption is enabled (even though it's not)
	cr := reflect.ValueOf(client).Elem().FieldByName("tls")
	cr = reflect.NewAt(cr.Type(), unsafe.Pointer(cr.UnsafeAddr())).Elem()
	cr.SetBool(true)

	// set the serverName field on the client which is used to identify the server and has to equal the hostname
	cr = reflect.ValueOf(client).Elem().FieldByName("serverName")
	cr = reflect.NewAt(cr.Type(), unsafe.Pointer(cr.UnsafeAddr())).Elem()
	cr.SetString(hostname)
}

// matchFailure is a simple wrapper around `fail` and `gomega.MatchError`` to make it easier to use in tests
func matchFailure(id failures.FailureID) gt.GomegaMatcher {
	return MatchError(fail(id, nil))
}
