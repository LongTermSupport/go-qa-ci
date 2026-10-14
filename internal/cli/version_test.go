package cli_test

import (
	"runtime/debug"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/cli"
)

func TestVersionIsTheModuleVersionTheBinaryWasBuiltFrom(t *testing.T) {
	for name, tc := range map[string]struct {
		info *debug.BuildInfo
		want string
	}{
		"installed at a tag":    {&debug.BuildInfo{Main: debug.Module{Version: "v1.2.3"}}, "v1.2.3"},
		"a pseudo-version":      {&debug.BuildInfo{Main: debug.Module{Version: "v0.0.0-20260101000000-abcdef123456"}}, "v0.0.0-20260101000000-abcdef123456"},
		"built from a checkout": {&debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, cli.DevVersion},
		"no module version":     {&debug.BuildInfo{}, cli.DevVersion},
		"no build information":  {nil, cli.DevVersion},
	} {
		t.Run(name, func(t *testing.T) {
			if got := cli.VersionFrom(tc.info); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
