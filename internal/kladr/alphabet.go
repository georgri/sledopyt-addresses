package kladr

import (
	"strings"
	"unicode"
)

var russianAlphabet = []rune{
	'а', 'б', 'в', 'г', 'д', 'е', 'ё', 'ж', 'з', 'и', 'й', 'к', 'л', 'м', 'н', 'о', 'п',
	'р', 'с', 'т', 'у', 'ф', 'х', 'ц', 'ч', 'ш', 'щ', 'ъ', 'ы', 'ь', 'э', 'ю', 'я',
}

var letterRank map[rune]int

func init() {
	letterRank = make(map[rune]int, len(russianAlphabet))
	for i, r := range russianAlphabet {
		letterRank[r] = i + 1
	}
}

func streetLetterValues(street string) []int {
	normalized := normalizeStreet(street)
	out := make([]int, 0, len(normalized))
	for _, r := range normalized {
		if rank, ok := letterRank[r]; ok {
			out = append(out, rank)
			continue
		}
		if r >= '0' && r <= '9' {
			out = append(out, int(r-'0'))
		}
	}
	return out
}

func normalizeStreet(street string) string {
	lowered := strings.ToLower(street)
	var b strings.Builder
	b.Grow(len(lowered))
	for _, r := range lowered {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
