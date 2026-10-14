package clean

import (
	"fmt"
	"slices"
	"sort"
)

func sortedAfter(m map[string]int) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func slicesSortedAfter(m map[string]int) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func sortSliceAfter(m map[string]int) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

func printsSortedKeys(m map[string]int) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		fmt.Println(k, m[k])
	}
}

// The slice never leaves the function, so its order is not observable.
func countsOnly(m map[string]int) int {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	return len(keys)
}

func sums(m map[string]int) int {
	total := 0
	for _, v := range m {
		total += v
	}
	return total
}

func overASlice(items []string) []string {
	var out []string
	for _, item := range items {
		out = append(out, item)
	}
	return out
}

func buildsAnotherMap(m map[string]int) map[int]string {
	out := map[int]string{}
	for k, v := range m {
		out[v] = k
	}
	return out
}
