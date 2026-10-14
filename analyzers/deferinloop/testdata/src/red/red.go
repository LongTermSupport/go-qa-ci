package red

import "os"

func inFor(paths []string) {
	for _, p := range paths {
		f, _ := os.Open(p)
		defer f.Close() // want `defer inside a loop`
	}
}

func inRangeOverInt() {
	for range 3 {
		defer println("x") // want `defer inside a loop`
	}
}

func inNestedBlocks(ok bool) {
	for i := 0; i < 3; i++ {
		if ok {
			switch i {
			case 1:
				defer println("x") // want `defer inside a loop`
			}
		}
	}
}

func inClosureLoop() {
	func() {
		for {
			defer println("x") // want `defer inside a loop`
			break
		}
	}()
}
