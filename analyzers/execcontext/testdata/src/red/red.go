package red

import (
	"context"
	"os/exec"
	osexec "os/exec"
	"testing"
)

func takesContext(ctx context.Context) error {
	return exec.Command("ls").Run() // want `exec\.Command is used while context ctx is in scope`
}

func aliasedImport(ctx context.Context) error {
	return osexec.Command("ls").Run() // want `exec\.Command is used while context ctx is in scope`
}

func storedInAVariable(ctx context.Context) error {
	build := exec.Command // want `exec\.Command is used while context ctx is in scope`
	return build("ls").Run()
}

func closureSeesTheContext(ctx context.Context) func() error {
	return func() error {
		return exec.Command("ls").Run() // want `exec\.Command is used while context ctx is in scope`
	}
}

func contextIsALocal() error {
	ctx := context.Background()
	_ = ctx
	return exec.Command("ls").Run() // want `exec\.Command is used while context ctx is in scope`
}

type runner struct{}

func (runner) run(ctx context.Context) error {
	return exec.Command("ls").Run() // want `exec\.Command is used while context ctx is in scope`
}

func TestSomething(t *testing.T) {
	_ = exec.Command("ls") // want `exec\.Command is used while context t is in scope`
}
