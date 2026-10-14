package sensitiveformat

import (
	"reflect"
	"testing"
)

func TestVerbsMapEachArgumentToItsVerb(t *testing.T) {
	cases := []struct {
		format string
		want   map[int]rune
	}{
		{"%v", map[int]rune{0: 'v'}},
		{"%d %s", map[int]rune{0: 'd', 1: 's'}},
		{"100%% %v", map[int]rune{0: 'v'}},
		{"%+v %#v", map[int]rune{0: 'v', 1: 'v'}},
		{"%-10.3s|%v", map[int]rune{0: 's', 1: 'v'}},
		{"%*d %v", map[int]rune{0: 'd', 1: 'd', 2: 'v'}},
		{"%[2]v %[1]s", map[int]rune{1: 'v', 0: 's'}},
		{"%[1]s %[1]v", map[int]rune{0: 'v'}},
		{"%", map[int]rune{}},
		{"%!", map[int]rune{0: '!'}},
	}
	for _, tc := range cases {
		got := verbsByArg(tc.format)
		// Only the verbs of interest matter for a repeated index, so compare the sets of verbs.
		for arg, verb := range tc.want {
			if !contains(got[arg], verb) {
				t.Errorf("%q: argument %d verbs %q, want %q", tc.format, arg, string(got[arg]), string(verb))
			}
		}
	}
}

func TestVerbsKeepEveryVerbOfARepeatedIndex(t *testing.T) {
	got := verbsByArg("%[1]s %[1]v")
	if !reflect.DeepEqual(got[0], []rune{'s', 'v'}) {
		t.Errorf("got %q", string(got[0]))
	}
}

func contains(verbs []rune, verb rune) bool {
	for _, v := range verbs {
		if v == verb {
			return true
		}
	}
	return false
}
