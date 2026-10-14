package main

import (
	"log"
	"os"
	osalias "os"
	"syscall"
)

func main() {
	run()
	os.Exit(0)
}

func run() {
	os.Exit(1)       // want `os\.Exit ends the process`
	osalias.Exit(1)  // want `os\.Exit ends the process`
	log.Fatal("x")   // want `log\.Fatal ends the process`
	log.Fatalf("x")  // want `log\.Fatalf ends the process`
	log.Fatalln("x") // want `log\.Fatalln ends the process`
	syscall.Exit(1)  // want `syscall\.Exit ends the process`
	logger := log.Default()
	logger.Fatal("x") // want `Logger\.Fatal ends the process`
}

var quit = os.Exit // want `os\.Exit ends the process`

var exitOnLoad = func() { os.Exit(2) } // want `os\.Exit ends the process`

type thing struct{}

func (thing) main() {
	os.Exit(3) // want `os\.Exit ends the process`
}
