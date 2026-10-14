package clean

import (
	"context"
	"os/exec"
)

func usesTheContext(ctx context.Context) error {
	return exec.CommandContext(ctx, "ls").Run()
}

// No context is in scope: nothing to prefer.
func noContext() error {
	return exec.Command("ls").Run()
}

// The context belongs to another function.
func other(ctx context.Context) {
	_ = ctx
}

func unrelated() error {
	return exec.Command("ls").Run()
}

func lookPath(ctx context.Context) (string, error) {
	_ = ctx
	return exec.LookPath("ls")
}
