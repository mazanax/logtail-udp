package version

import (
	"runtime/debug"
)

// Version returns actual version of application. Usually it's a git hash but in dev environment this function returns "dev"
func Version() string {
	b, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}

	for _, kv := range b.Settings {
		if kv.Key == "vcs.revision" {
			return kv.Value
		}
	}

	return "dev"
}
