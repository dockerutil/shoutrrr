package util_test

import (
	"fmt"
	"reflect"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dockerutil/shoutrrr/internal/meta"
	. "github.com/dockerutil/shoutrrr/pkg/util"
)

func TestUtil(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Shoutrrr Util Suite")
}

const a = 10
const b = 20

var _ = Describe("the util package", func() {
	When("calling function Min", func() {
		It("should return the smallest of two integers", func() {
			Expect(Min(a, b)).To(Equal(a))
			Expect(Min(b, a)).To(Equal(a))
		})
	})

	When("calling function Max", func() {
		It("should return the largest of two integers", func() {
			Expect(Max(a, b)).To(Equal(b))
			Expect(Max(b, a)).To(Equal(b))
		})
	})

	When("checking if a supplied kind is of the signed integer kind", func() {
		It("should be true if the kind is Int", func() {
			Expect(IsSignedInt(reflect.Int)).To(BeTrue())
		})
		It("should be false if the kind is String", func() {
			Expect(IsSignedInt(reflect.String)).To(BeFalse())
		})
	})

	When("checking if a supplied kind is of the unsigned integer kind", func() {
		It("should be true if the kind is Uint", func() {
			Expect(IsUnsignedInt(reflect.Uint)).To(BeTrue())
		})
		It("should be false if the kind is Int", func() {
			Expect(IsUnsignedInt(reflect.Int)).To(BeFalse())
		})
	})

	When("checking if a supplied kind is of the collection kind", func() {
		It("should be true if the kind is slice", func() {
			Expect(IsCollection(reflect.Slice)).To(BeTrue())
		})
		It("should be false if the kind is map", func() {
			Expect(IsCollection(reflect.Map)).To(BeFalse())
		})
	})

	When("calling function StripNumberPrefix", func() {
		It("should return the default base if none is found", func() {
			_, base := StripNumberPrefix("46")
			Expect(base).To(Equal(0))
		})
		It("should remove # prefix and return base 16 if found", func() {
			number, base := StripNumberPrefix("#ab")
			Expect(number).To(Equal("ab"))
			Expect(base).To(Equal(16))
		})
	})

	When("checking if a supplied kind is numeric", func() {
		It("should be true if supplied a constant integer", func() {
			Expect(IsNumeric(reflect.TypeOf(5).Kind())).To(BeTrue())
		})
		It("should be true if supplied a constant float", func() {
			Expect(IsNumeric(reflect.TypeOf(2.5).Kind())).To(BeTrue())
		})
		It("should be false if supplied a constant string", func() {
			Expect(IsNumeric(reflect.TypeOf("3").Kind())).To(BeFalse())
		})
	})

	When("calling function DocsURL", func() {
		It("should return the expected URL", func() {
			expectedBase := fmt.Sprintf(`https://containrrr.dev/shoutrrr/%s/`, meta.DocsVersion)
			Expect(DocsURL(``)).To(Equal(expectedBase))
			Expect(DocsURL(`services/logger`)).To(Equal(expectedBase + `services/logger`))
		})
		It("should strip the leading slash from the path", func() {
			Expect(DocsURL(`/foo`)).To(Equal(DocsURL(`foo`)))
		})
	})
})
