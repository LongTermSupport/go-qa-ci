package red

import (
	"fmt"
	"io"
	"os"
	"strings"
)

func printsInMapOrder(m map[string]int) {
	for k, v := range m { // want `ranges over a map and writes output in map order`
		fmt.Println(k, v)
	}
}

func writesInMapOrder(w io.Writer, m map[string]int) {
	for k := range m { // want `ranges over a map and writes output in map order`
		fmt.Fprintf(w, "%s\n", k)
	}
}

func builderInMapOrder(m map[string]int) string {
	var b strings.Builder
	for k := range m { // want `ranges over a map and writes output in map order`
		b.WriteString(k)
	}
	return b.String()
}

func fileInMapOrder(f *os.File, m map[string][]byte) {
	for _, v := range m { // want `ranges over a map and writes output in map order`
		_, _ = f.Write(v)
	}
}

func returnsMapOrder(m map[string]int) []string {
	var keys []string
	for k := range m { // want `ranges over a map and appends to keys, which is never sorted before it is returned or printed`
		keys = append(keys, k)
	}
	return keys
}

func joinsMapOrder(m map[string]int) string {
	var keys []string
	for k := range m { // want `ranges over a map and appends to keys, which is never sorted before it is returned or printed`
		keys = append(keys, k)
	}
	return strings.Join(keys, ",")
}

func namedResult(m map[string]int) (out []string) {
	for k := range m { // want `ranges over a map and appends to out, which is never sorted before it is returned or printed`
		out = append(out, k)
	}
	return
}

type Set map[string]struct{}

func definedMapType(s Set) []string {
	var out []string
	for k := range s { // want `ranges over a map and appends to out, which is never sorted before it is returned or printed`
		out = append(out, k)
	}
	return out
}
