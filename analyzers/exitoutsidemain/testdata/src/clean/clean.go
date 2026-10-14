package main

import (
	"log"
	"os"
)

func main() {
	if len(os.Args) > 5 {
		log.Fatal("too many")
	}
	defer func() { os.Exit(1) }()
	os.Exit(run())
}

func run() int {
	log.Println("ok")
	return 0
}
