package clean

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var (
	pattern  *regexp.Regexp
	table    map[string]int
	home     string
	failure  = errors.New("x")
	handlers []func()
)

func init() {
	pattern = regexp.MustCompile(`^a+$`)
	table = map[string]int{"a": 1}
	home = os.Getenv("HOME")
	if v, ok := os.LookupEnv("X"); ok {
		home = v
	}
	handlers = append(handlers, func() { _ = os.Remove("never called at init") })
	table[strings.ToUpper("b")] = len(fmt.Sprintf("%d", 1))
}

func notInit() {
	_ = os.Setenv("A", "b")
	go notInit()
}
