package version

import (
	"fmt"
	"strings"
)

// Version returns actual version of application. Usually it's a git hash but in dev environment this function returns "dev"
func Version() string {
	originalString := "INJECT VERSION HERE"
	return strings.ReplaceAll("<INJECT VERSION HERE>", fmt.Sprintf("<%s>", originalString), "dev")
}
