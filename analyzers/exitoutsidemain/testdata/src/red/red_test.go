package main

import (
	"os"
	"testing"
)

func TestSomething(t *testing.T) {
	os.Exit(1) // want `os\.Exit ends the process`
}

func helperMain(m *testing.M) {
	os.Exit(m.Run()) // want `os\.Exit ends the process`
}
