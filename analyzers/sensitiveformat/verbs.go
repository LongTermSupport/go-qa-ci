package sensitiveformat

import (
	"strconv"
	"strings"
)

// verbsByArg maps the index of each operand of a printf-style format to the verbs that format it,
// following fmt's own rules for flags, width, precision, '*' and explicit [n] argument indexes.
// An operand named by two verbs, as in "%[1]s %[1]v", carries both.
func verbsByArg(format string) map[int][]rune {
	verbs := map[int][]rune{}
	arg := 0
	runes := []rune(format)
	for i := 0; i < len(runes); i++ {
		if runes[i] != '%' {
			continue
		}
		i++
		i = skipFlags(runes, i)
		arg, i = applyIndex(runes, i, arg)
		if i < len(runes) && runes[i] == '*' {
			verbs[arg] = append(verbs[arg], 'd')
			arg++
			i++
		} else {
			i = skipDigits(runes, i)
		}
		if i < len(runes) && runes[i] == '.' {
			i++
			arg, i = applyIndex(runes, i, arg)
			if i < len(runes) && runes[i] == '*' {
				verbs[arg] = append(verbs[arg], 'd')
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
		verbs[arg] = append(verbs[arg], runes[i])
		arg++
	}
	return verbs
}

func skipFlags(runes []rune, i int) int {
	for i < len(runes) && strings.ContainsRune("+-# 0", runes[i]) {
		i++
	}
	return i
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
