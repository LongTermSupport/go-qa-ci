package sensitiveformat

import (
	"strconv"
	"strings"
)

// verb is one conversion of a printf-style format: its letter and whether the '#' flag was given,
// which is what makes %#v Go-syntax formatting.
type verb struct {
	letter rune
	sharp  bool
}

// verbsByArg maps the index of each operand of a printf-style format to the verbs that format it,
// following fmt's own rules for flags, width, precision, '*' and explicit [n] argument indexes.
// An operand named by two verbs, as in "%[1]s %[1]v", carries both.
func verbsByArg(format string) map[int][]verb {
	verbs := map[int][]verb{}
	arg := 0
	runes := []rune(format)
	for i := 0; i < len(runes); i++ {
		if runes[i] != '%' {
			continue
		}
		i++
		var sharp bool
		sharp, i = readFlags(runes, i)
		arg, i = applyIndex(runes, i, arg)
		if i < len(runes) && runes[i] == '*' {
			verbs[arg] = append(verbs[arg], verb{letter: 'd'})
			arg++
			i++
		} else {
			i = skipDigits(runes, i)
		}
		if i < len(runes) && runes[i] == '.' {
			i++
			arg, i = applyIndex(runes, i, arg)
			if i < len(runes) && runes[i] == '*' {
				verbs[arg] = append(verbs[arg], verb{letter: 'd'})
				arg++
				i++
			} else {
				i = skipDigits(runes, i)
			}
		}
		arg, i = applyIndex(runes, i, arg)
		if i >= len(runes) {
			break
		}
		if runes[i] == '%' {
			continue
		}
		verbs[arg] = append(verbs[arg], verb{letter: runes[i], sharp: sharp})
		arg++
	}
	return verbs
}

// readFlags consumes the flags at i and reports whether '#' was among them.
func readFlags(runes []rune, i int) (sharp bool, next int) {
	for i < len(runes) && strings.ContainsRune("+-# 0", runes[i]) {
		sharp = sharp || runes[i] == '#'
		i++
	}
	return sharp, i
}

func skipDigits(runes []rune, i int) int {
	for i < len(runes) && runes[i] >= '0' && runes[i] <= '9' {
		i++
	}
	return i
}

// applyIndex consumes an explicit "[n]" at i and returns the operand index it selects.
func applyIndex(runes []rune, i, arg int) (nextArg, nextPos int) {
	if i >= len(runes) || runes[i] != '[' {
		return arg, i
	}
	end := i + 1
	for end < len(runes) && runes[end] != ']' {
		end++
	}
	if end >= len(runes) {
		return arg, i
	}
	n, err := strconv.Atoi(string(runes[i+1 : end]))
	if err != nil || n < 1 {
		return arg, end + 1
	}
	return n - 1, end + 1
}
