package generic

import (
	"errors"
	"github.com/dockerutil/shoutrrr/internal/testutils"
	"io"
	"log"
	"net/url"
	"testing"

	"github.com/dockerutil/shoutrrr/pkg/format"
	"github.com/dockerutil/shoutrrr/pkg/types"
	"github.com/jarcoal/httpmock"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/ghttp"
)

func TestGeneric(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Shoutrrr Generic Webhook Suite")
}

var (
	logger  = log.New(GinkgoWriter, "Test", log.LstdFlags)
	service *Service
)

var _ = Describe("the Generic service", func() {
	BeforeEach(func() {
		service = &Service{}
		service.SetLogger(logger)
	})
	When("parsing a custom URL", func() {
		It("should strip generic prefix before parsing", func() {
			customURL, err := url.Parse("generic+https://test.tld")
			Expect(err).NotTo(HaveOccurred())
			actualURL, err := service.GetConfigURLFromCustom(customURL)
			Expect(err).NotTo(HaveOccurred())
			_, expectedURL := testCustomURL("https://test.tld")
			Expect(actualURL.String()).To(Equal(expectedURL.String()))
		})

		When("a HTTP URL is provided", func() {
			It("should disable TLS", func() {
				config, _ := testCustomURL("http://example.com")
				Expect(config.DisableTLS).To(BeTrue())
			})
		})
		When("a HTTPS URL is provided", func() {
			It("should enable TLS", func() {
				config, _ := testCustomURL("https://example.com")
				Expect(config.DisableTLS).To(BeFalse())
			})
		})
		It("should escape conflicting custom query keys", func() {
			expectedURL := "generic://example.com/?__template=passed"
			config, srvURL := testCustomURL("https://example.com/?template=passed")
			Expect(config.Template).NotTo(Equal("passed")) // captured
			whURL := config.WebhookURL().String()
			Expect(whURL).To(Equal("https://example.com/?template=passed"))
			Expect(srvURL.String()).To(Equal(expectedURL))

		})
		It("should handle both escaped and service prop version of keys", func() {
			config, _ := testServiceURL("generic://example.com/?__template=passed&template=captured")
			Expect(config.Template).To(Equal("captured"))
			whURL := config.WebhookURL().String()
			Expect(whURL).To(Equal("https://example.com/?template=passed"))
		})

		When("the URL includes custom headers", func() {
			It("should strip the headers from the webhook query", func() {
				config, _ := testServiceURL("generic://example.com/?@authorization=frend")
				Expect(config.WebhookURL().Query()).NotTo(HaveKey("@authorization"))
				Expect(config.WebhookURL().Query()).NotTo(HaveKey("authorization"))
			})
			It("should add the headers to the config custom header map", func() {
				config, _ := testServiceURL("generic://example.com/?@authorization=frend")
				Expect(config.headers).To(HaveKeyWithValue("Authorization", "frend"))
			})
			When("header keys are in camelCase", func() {
				It("should add headers with kebab-case keys", func() {
					config, _ := testServiceURL("generic://example.com/?@userAgent=gozilla+1.0")
					Expect(config.headers).To(HaveKeyWithValue("User-Agent", "gozilla 1.0"))
				})
			})
		})

		When("the URL includes extra data", func() {
			It("should strip the extra data from the webhook query", func() {
				config, _ := testServiceURL("generic://example.com/?$context=inside+joke")
				Expect(config.WebhookURL().Query()).NotTo(HaveKey("$context"))
				Expect(config.WebhookURL().Query()).NotTo(HaveKey("context"))
			})
			It("should add the extra data to the config extra data map", func() {
				config, _ := testServiceURL("generic://example.com/?$context=inside+joke")
				Expect(config.extraData).To(HaveKeyWithValue("context", "inside joke"))
			})
		})
	})
	When("retrieving the webhook URL", func() {
		It("should build a valid webhook URL", func() {
			expectedURL := "https://example.com/path?foo=bar"
			config, _ := testServiceURL("generic://example.com/path?foo=bar")
			Expect(config.WebhookURL().String()).To(Equal(expectedURL))
		})

		When("TLS is disabled", func() {
			It("should use http schema", func() {
				config := Config{
					webhookURL: &url.URL{
						Host: "test.tld",
					},
					DisableTLS: true,
				}
				Expect(config.WebhookURL().Scheme).To(Equal("http"))
			})
		})
		When("TLS is not disabled", func() {
			It("should use https schema", func() {
				config := Config{
					webhookURL: &url.URL{
						Host: "test.tld",
					},
					DisableTLS: false,
				}
				Expect(config.WebhookURL().Scheme).To(Equal("https"))
			})
		})
	})

	Describe("creating a config", func() {
		When("creating a default config", func() {
			It("should not return an error", func() {
				config := &Config{}
				pkr := format.NewPropKeyResolver(config)
				err := pkr.SetDefaultProps(config)
				Expect(err).NotTo(HaveOccurred())
			})
		})
		When("parsing the configuration URL", func() {
			It("should be identical after de-/serialization", func() {
				testURL := "generic://user:pass@host.tld/api/v1/webhook?%24context=inside-joke&%40Authorization=frend&__title=w&contenttype=a%2Fb&template=f&title=t"

				url, err := url.Parse(testURL)
				Expect(err).NotTo(HaveOccurred(), "parsing")

				config := &Config{}
				pkr := format.NewPropKeyResolver(config)
				Expect(pkr.SetDefaultProps(config)).To(Succeed())
				err = config.SetURL(url)
				Expect(err).NotTo(HaveOccurred(), "verifying")

				outputURL := config.GetURL()
				Expect(outputURL.String()).To(Equal(testURL))

			})
		})
	})

	Describe("building the payload", func() {
		var service Service
		var config Config
		BeforeEach(func() {
			service = Service{}
			config = Config{
				MessageKey: "message",
				TitleKey:   "title",
			}
		})
		When("no template is specified", func() {
			It("should use the message as payload", func() {
				payload, err := service.getPayload(&config, types.Params{"message": "test message"})
				Expect(err).NotTo(HaveOccurred())
				contents, err := io.ReadAll(payload)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("test message"))
			})
		})
		When("template is specified as `JSON`", func() {
			It("should create a JSON object as the payload", func() {
				config.Template = "JSON"
				params := types.Params{"title": "test title"}
				sendParams := createSendParams(&config, params, "test message")
				payload, err := service.getPayload(&config, sendParams)
				Expect(err).NotTo(HaveOccurred())
				contents, err := io.ReadAll(payload)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(MatchJSON(`{
					"title":   "test title",
					"message": "test message"
				}`))
			})
			When("alternate keys are specified", func() {
				It("should create a JSON object using the specified keys", func() {
					config.Template = "JSON"
					config.MessageKey = "body"
					config.TitleKey = "header"
					params := types.Params{"title": "test title"}
					sendParams := createSendParams(&config, params, "test message")
					payload, err := service.getPayload(&config, sendParams)
					Expect(err).NotTo(HaveOccurred())
					contents, err := io.ReadAll(payload)
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(MatchJSON(`{
						"header":   "test title",
						"body": "test message"
					}`))
				})
			})
		})
		When("a valid template is specified", func() {
			It("should apply the template to the message payload", func() {
				err := service.SetTemplateString("news", `{{.title}} ==> {{.message}}`)
				Expect(err).NotTo(HaveOccurred())
				params := types.Params{}
				params.SetTitle("BREAKING NEWS")
				params.SetMessage("it's today!")
				config.Template = "news"
				payload, err := service.getPayload(&config, params)
				Expect(err).NotTo(HaveOccurred())
				contents, err := io.ReadAll(payload)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("BREAKING NEWS ==> it's today!"))
			})
			When("given nil params", func() {
				It("should apply template with message data", func() {
					err := service.SetTemplateString("arrows", `==> {{.message}} <==`)
					Expect(err).NotTo(HaveOccurred())
					config.Template = "arrows"
					payload, err := service.getPayload(&config, types.Params{"message": "LOOK AT ME"})
					Expect(err).NotTo(HaveOccurred())
					contents, err := io.ReadAll(payload)
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal("==> LOOK AT ME <=="))
				})
			})
		})
		When("an unknown template is specified", func() {
			It("should return an error", func() {
				_, err := service.getPayload(&Config{Template: "missing"}, nil)
				Expect(err).To(HaveOccurred())
			})
		})

	})
	Describe("sending the payload", func() {
		var err error
		var service Service
		BeforeEach(func() {
			httpmock.Activate()
		})
		AfterEach(func() {
			httpmock.DeactivateAndReset()
		})

		It("should not report an error if the server accepts the payload", func() {
			serviceURL, _ := url.Parse("generic://host.tld/webhook")
			err = service.Initialize(serviceURL, logger)
			Expect(err).NotTo(HaveOccurred())

			targetURL := "https://host.tld/webhook"
			httpmock.RegisterResponder("POST", targetURL, httpmock.NewStringResponder(200, ""))

			err = service.Send("Message", nil)
			Expect(err).NotTo(HaveOccurred())
		})
		It("should not panic if an error occurs when sending the payload", func() {
			serviceURL, _ := url.Parse("generic://host.tld/webhook")
			err = service.Initialize(serviceURL, logger)
			Expect(err).NotTo(HaveOccurred())

			targetURL := "https://host.tld/webhook"
			httpmock.RegisterResponder("POST", targetURL, httpmock.NewErrorResponder(errors.New("dummy error")))

			err = service.Send("Message", nil)
			Expect(err).To(HaveOccurred())
		})
		It("should not return an error when an unknown param is encountered", func() {
			serviceURL, _ := url.Parse("generic://host.tld/webhook")
			err = service.Initialize(serviceURL, logger)
			Expect(err).NotTo(HaveOccurred())

			targetURL := "https://host.tld/webhook"
			httpmock.RegisterResponder("POST", targetURL, httpmock.NewStringResponder(200, ""))

			err = service.Send("Message", &types.Params{"unknown": "param"})
			Expect(err).NotTo(HaveOccurred())
		})
		It("should use the configured HTTP method", func() {
			serviceURL, _ := url.Parse("generic://host.tld/webhook?method=GET")
			err = service.Initialize(serviceURL, logger)
			Expect(err).NotTo(HaveOccurred())

			targetURL := "https://host.tld/webhook"
			httpmock.RegisterResponder("GET", targetURL, httpmock.NewStringResponder(200, ""))

			err = service.Send("Message", nil)
			Expect(err).NotTo(HaveOccurred())
		})
		It("should not mutate the given params", func() {
			serviceURL, _ := url.Parse("generic://host.tld/webhook?method=GET")
			err = service.Initialize(serviceURL, logger)
			Expect(err).NotTo(HaveOccurred())

			targetURL := "https://host.tld/webhook"
			httpmock.RegisterResponder("GET", targetURL, httpmock.NewStringResponder(200, ""))

			params := types.Params{"title": "TITLE"}

			err = service.Send("Message", &params)
			Expect(err).NotTo(HaveOccurred())

			Expect(params).To(Equal(types.Params{"title": "TITLE"}))
		})

	})
	Describe("the service upstream client", func() {
		var server *ghttp.Server
		var serverHost string
		BeforeEach(func() {
			server = ghttp.NewServer()
			serverHost = testutils.URLMust(server.URL()).Host
		})
		AfterEach(func() {
			server.Close()
		})

		When("custom headers are configured", func() {
			It("should add those headers to the request", func() {
				serviceURL := testutils.URLMust("generic://host.tld/webhook?disabletls=yes&@authorization=frend&@userAgent=gozilla+1.0")
				serviceURL.Host = serverHost
				Expect(service.Initialize(serviceURL, logger)).NotTo(HaveOccurred())

				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/webhook"),
						ghttp.VerifyHeaderKV("authorization", "frend"),
						ghttp.VerifyHeaderKV("user-agent", "gozilla 1.0"),
					),
				)

				Expect(service.Send("Message", nil)).NotTo(HaveOccurred())
			})
		})

		When("extra data is configured", func() {
			When("json template is used", func() {
				It("should add those extra data fields to the request", func() {
					serviceURL := testutils.URLMust("generic://host.tld/webhook?disabletls=yes&template=json&$context=inside+joke")
					serviceURL.Host = serverHost
					Expect(service.Initialize(serviceURL, logger)).NotTo(HaveOccurred())

					server.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", "/webhook"),
							ghttp.VerifyJSONRepresenting(map[string]string{
								"message": "Message",
								"context": "inside joke",
							}),
						),
					)

					Expect(service.Send("Message", nil)).NotTo(HaveOccurred())
				})
			})
		})
	})
	Describe("the normalized header key format", func() {
		It("should match the format", func() {
			Expect(normalizedHeaderKey("content-type")).To(Equal("Content-Type"))
		})
		It("should match the format", func() {
			Expect(normalizedHeaderKey("contentType")).To(Equal("Content-Type"))
		})
		It("should match the format", func() {
			Expect(normalizedHeaderKey("ContentType")).To(Equal("Content-Type"))
		})
		It("should match the format", func() {
			Expect(normalizedHeaderKey("Content-Type")).To(Equal("Content-Type"))
		})
	})
})

func testCustomURL(testURL string) (*Config, *url.URL) {
	customURL, err := url.Parse(testURL)
	Expect(err).NotTo(HaveOccurred())
	config, pkr, err := ConfigFromWebhookURL(*customURL)
	Expect(err).NotTo(HaveOccurred())
	return config, config.getURL(&pkr)
}

func testServiceURL(testURL string) (*Config, *url.URL) {
	serviceURL, err := url.Parse(testURL)
	Expect(err).NotTo(HaveOccurred())
	config, pkr := DefaultConfig()
	err = config.setURL(&pkr, serviceURL)
	Expect(err).NotTo(HaveOccurred())
	return config, config.getURL(&pkr)
}
