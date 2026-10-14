package sensitiveformat

import (
	"slices"
	"testing"
)

func TestVerbsMapEachArgumentToItsVerb(t *testing.T) {
	cases := []struct {
		format string
		want   map[int]verb
	}{
		{"%v", map[int]verb{0: {letter: 'v'}}},
		{"%d %s", map[int]verb{0: {letter: 'd'}, 1: {letter: 's'}}},
		{"100%% %v", map[int]verb{0: {letter: 'v'}}},
		{"%+v %#v", map[int]verb{0: {letter: 'v'}, 1: {letter: 'v', sharp: true}}},
		{"%-10.3s|%v", map[int]verb{0: {letter: 's'}, 1: {letter: 'v'}}},
		{"%*d %v", map[int]verb{0: {letter: 'd'}, 1: {letter: 'd'}, 2: {letter: 'v'}}},
		{"%[2]v %[1]s", map[int]verb{1: {letter: 'v'}, 0: {letter: 's'}}},
		{"%[1]s %[1]v", map[int]verb{0: {letter: 'v'}}},
		{"%", map[int]verb{}},
		{"%!", map[int]verb{0: {letter: '!'}}},
	}
	for _, tc := range cases {
		got := verbsByArg(tc.format)
		// Only the verbs of interest matter for a repeated index, so compare membership.
		for arg, want := range tc.want {
			if !slices.Contains(got[arg], want) {
				t.Errorf("%q: argument %d verbs %v, want %v", tc.format, arg, got[arg], want)
			}
		}
	}
}

func TestVerbsKeepEveryVerbOfARepeatedIndex(t *testing.T) {
	got := verbsByArg("%[1]s %[1]v")
	if !slices.Equal(got[0], []verb{{letter: 's'}, {letter: 'v'}}) {
		t.Errorf("got %v", got[0])
	}
}
