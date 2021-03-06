package configuration_test

import (
	. "cf/configuration"
	"cf/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"regexp"
)

var exampleJSON = `
{
	"ConfigVersion": 2,
	"Target": "api.example.com",
	"ApiVersion": "2",
	"AuthorizationEndpoint": "auth.example.com",
	"LoggregatorEndpoint": "logs.example.com",
	"AccessToken": "the-access-token",
	"RefreshToken": "the-refresh-token",
	"OrganizationFields": {
		"Guid": "the-org-guid",
		"Name": "the-org",
		"QuotaDefinition": {
			"Guid": "",
			"Name": "",
			"MemoryLimit": 0
		}
	},
	"SpaceFields": {
		"Guid": "the-space-guid",
		"Name": "the-space"
	}
}`

var exampleConfig = &Data{
	Target:                "api.example.com",
	ApiVersion:            "2",
	AuthorizationEndpoint: "auth.example.com",
	LoggregatorEndPoint:   "logs.example.com",
	AccessToken:           "the-access-token",
	RefreshToken:          "the-refresh-token",
	OrganizationFields: models.OrganizationFields{
		Guid: "the-org-guid",
		Name: "the-org",
	},
	SpaceFields: models.SpaceFields{
		Guid: "the-space-guid",
		Name: "the-space",
	},
}

var _ = Describe("V2 Config files", func() {
	Describe("serialization", func() {
		It("creates a JSON string from the config object", func() {
			jsonData, err := JsonMarshalV2(exampleConfig)

			Expect(err).NotTo(HaveOccurred())
			Expect(stripWhitespace(string(jsonData))).To(Equal(stripWhitespace(exampleJSON)))
		})
	})

	Describe("parsing", func() {
		It("returns an error when the JSON is invalid", func() {
			configData := NewData()
			err := JsonUnmarshalV2([]byte(`{ "not_valid": ### }`), configData)

			Expect(err).To(HaveOccurred())
		})

		It("creates a config object from valid JSON", func() {
			configData := NewData()
			err := JsonUnmarshalV2([]byte(exampleJSON), configData)

			Expect(err).NotTo(HaveOccurred())
			Expect(configData).To(Equal(exampleConfig))
		})
	})
})

var whiteSpaceRegex = regexp.MustCompile(`\s+`)

func stripWhitespace(input string) string {
	return whiteSpaceRegex.ReplaceAllString(input, "")
}
