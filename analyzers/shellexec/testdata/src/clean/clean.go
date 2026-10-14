package clean

import "os/exec"

func scriptFile(args []string) {
	_ = exec.Command("sh", "script.sh")
	_ = exec.Command("bash", "script.sh", "-c")
	_ = exec.Command("bash", "-e", "script.sh")
}

func noShell(cmd string) {
	_ = exec.Command("git", "-c", "user.name=x", "status")
	_ = exec.Command("python3", "-c", cmd)
	_ = exec.Command("ls", "-c")
	_ = exec.Command("env", "FOO=1", "git", "status")
}

// Not decidable here: the arguments are spread from a slice.
func spread(args []string) {
	_ = exec.Command("sh", args...)
}

func notConstant(shell, flag string) {
	_ = exec.Command(shell, flag, "ls")
}

func interactive() {
	_ = exec.Command("bash")
	_ = exec.Command("bash", "--norc")
}
