package red

import (
	"context"
	"os"
	"os/exec"
	"syscall"
)

const shell = "bash"

const flag = "-c"

func literal() {
	_ = exec.Command("sh", "-c", "ls | wc -l") // want `runs the shell sh with -c`
	_ = exec.Command("/bin/bash", "-c", "ls")  // want `runs the shell bash with -c`
	_ = exec.Command("zsh", "-c", "ls")        // want `runs the shell zsh with -c`
	_ = exec.Command("dash", "-c", "ls")       // want `runs the shell dash with -c`
}

func throughConstants(cmd string) {
	_ = exec.Command(shell, flag, cmd) // want `runs the shell bash with -c`
}

func clustered(cmd string) {
	_ = exec.Command("bash", "-lc", cmd)    // want `runs the shell bash with -lc`
	_ = exec.Command("sh", "-e", "-c", cmd) // want `runs the shell sh with -c`
}

func withContext(ctx context.Context, cmd string) {
	_ = exec.CommandContext(ctx, "sh", "-c", cmd) // want `runs the shell sh with -c`
}

func throughAWrapper(cmd string) {
	_ = exec.Command("env", "FOO=1", "sh", "-c", cmd) // want `runs the shell sh with -c`
	_ = exec.Command("sudo", "bash", "-c", cmd)       // want `runs the shell bash with -c`
}

func replacesTheProcess(cmd string) {
	_ = syscall.Exec("/bin/sh", []string{"sh", "-c", cmd}, nil)       // want `runs the shell sh with -c`
	_, _ = os.StartProcess("/bin/sh", []string{"sh", "-c", cmd}, nil) // want `runs the shell sh with -c`
}
