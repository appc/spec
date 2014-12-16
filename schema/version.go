package schema

import (
	"github.com/appc/spec/schema/types"
)

var (
	// VERSION represents the canonical version of the appc spec and tooling.
	// Must be set during build with:
	//   -ldflags "-X github.com/appc/spec/schema.VERSION $VERSION"
	VERSION string

	// AppContainerVersion is the SemVer representation of VERSION
	AppContainerVersion types.SemVer
)

func init() {
	if VERSION == "" {
		panic("VERSION must be set with ldflags")
	}
	v, err := types.NewSemVer(VERSION)
	if err != nil {
		panic(err)
	}
	AppContainerVersion = *v
}
