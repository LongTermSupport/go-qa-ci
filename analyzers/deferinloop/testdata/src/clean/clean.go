package clean

import "os"

func oncePerCall(path string) {
	f, _ := os.Open(path)
	defer f.Close()
}

// The deferred call belongs to the closure, which returns every iteration.
func perIteration(paths []string) {
	for _, p := range paths {
		func() {
			f, _ := os.Open(p)
			defer f.Close()
		}()
	}
}

func closureAssignedInLoop(paths []string) {
	for range paths {
		run := func() { defer println("x") }
		run()
	}
}

func beforeTheLoop() {
	defer println("x")
	for range 3 {
	}
}
