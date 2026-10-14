package library

import "os"

// A function named main in a library package is not the program entry point.
func main() {
	os.Exit(0) // want `os\.Exit ends the process`
}
