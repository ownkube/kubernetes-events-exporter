package buildinfo

import (
	"runtime"
	"runtime/debug"
)

var (
	Version   = "0.1.2"
	GoVersion = runtime.Version()
	GoOS      = runtime.GOOS
	GoArch    = runtime.GOARCH
)

func Revision() string {
	bi, ok := debug.ReadBuildInfo()

	if ok {
		for _, kv := range bi.Settings {
			switch kv.Key {
			case "vcs.revision":
				return kv.Value
			}
		}
	}

	return "unknown"
}
