package toolstore

import (
	"slices"
	"testing"
)

func TestGoInstallIgnoresTheConsumersVendoringAndWorkspace(t *testing.T) {
	env := goInstallEnv("/gobin")
	for _, want := range []string{"GOBIN=/gobin", "GOFLAGS=-mod=mod", "GOWORK=off"} {
		if !slices.Contains(env, want) {
			t.Errorf("%s missing from %v", want, env)
		}
	}
	for _, entry := range env {
		if entry == "GOTOOLCHAIN=local" || entry == "GOPROXY=off" {
			t.Errorf("%s must stay the consumer's to set", entry)
		}
	}
}
