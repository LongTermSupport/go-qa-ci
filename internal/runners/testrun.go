package runners

import (
	"context"
)

// testFlags run every test with the race detector, in random order and uncached.
var testFlags = []string{"test", "-race", "-shuffle=on", "-count=1", "./..."}

// Test runs the module's tests in dir. passed is false when a test failed or the code did not
// build; output is then everything go test printed.
func Test(ctx context.Context, dir string) (output string, passed bool, err error) {
	out, err := Capture(ctx, dir, goBinary, testFlags...)
	if err != nil {
		return "", false, err
	}
	return out.Combined(), out.Code == 0, nil
}
