package formula

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

var (
	reSpace      = regexp.MustCompile(`\s+`)
	reQuotedTail = regexp.MustCompile(`(?i)["“”']([а-яa-z])["“”']$`)
	reTerm       = regexp.MustCompile(`(?i)^([+-]?\d+)(?:\*x(\d+))?$`)
)

type Parsed struct {
	Coefficients map[int]int
	Constant     int
	Suffix       string
	Normalized   string
}

func Parse(raw string) (Parsed, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return Parsed{}, fmt.Errorf("formula is empty")
	}

	s = strings.ReplaceAll(s, "−", "-")
	s = strings.ReplaceAll(s, "—", "-")
	s = strings.ReplaceAll(s, "–", "-")
	s = strings.ReplaceAll(s, "х", "x")
	s = strings.ReplaceAll(s, "Х", "x")
	s = strings.ReplaceAll(s, "X", "x")
	s = reSpace.ReplaceAllString(s, "")

	suffix := ""
	if m := reQuotedTail.FindStringSubmatch(s); len(m) == 2 {
		suffix = strings.ToLower(m[1])
		s = strings.TrimSpace(strings.TrimSuffix(s, m[0]))
		s = strings.TrimSuffix(s, "+")
	}

	if s == "" {
		return Parsed{}, fmt.Errorf("formula has no numeric part")
	}

	parts := splitExpression(s)
	coeffs := make(map[int]int)
	constant := 0
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			return Parsed{}, fmt.Errorf("invalid formula syntax")
		}

		m := reTerm.FindStringSubmatch(p)
		if len(m) == 0 {
			return Parsed{}, fmt.Errorf("unsupported term: %s", p)
		}

		value, err := strconv.Atoi(m[1])
		if err != nil {
			return Parsed{}, fmt.Errorf("invalid number: %w", err)
		}

		if m[2] == "" {
			constant += value
			continue
		}

		i, err := strconv.Atoi(m[2])
		if err != nil || i <= 0 {
			return Parsed{}, fmt.Errorf("invalid xi index in term: %s", p)
		}
		coeffs[i] += value
	}

	normalized := buildNormalized(constant, coeffs, suffix)
	return Parsed{
		Coefficients: coeffs,
		Constant:     constant,
		Suffix:       suffix,
		Normalized:   normalized,
	}, nil
}

func (p Parsed) Eval(letters []int) (string, bool) {
	result := p.Constant
	for idx, coeff := range p.Coefficients {
		if idx < 1 || idx > len(letters) {
			return "", false
		}
		result += coeff * letters[idx-1]
	}

	if result <= 0 {
		return "", false
	}
	return strconv.Itoa(result) + p.Suffix, true
}

func splitExpression(s string) []string {
	var out []string
	start := 0
	for i := 1; i < len(s); i++ {
		if s[i] == '+' || s[i] == '-' {
			out = append(out, s[start:i])
			start = i
		}
	}
	out = append(out, s[start:])
	return out
}

func buildNormalized(constant int, coeffs map[int]int, suffix string) string {
	var terms []string
	if constant != 0 {
		terms = append(terms, strconv.Itoa(constant))
	}

	for i := 1; i <= 64; i++ {
		coeff, ok := coeffs[i]
		if !ok || coeff == 0 {
			continue
		}
		term := fmt.Sprintf("%d*x%d", coeff, i)
		terms = append(terms, term)
	}

	if len(terms) == 0 {
		terms = append(terms, "0")
	}

	expr := strings.Join(terms, " + ")
	expr = strings.ReplaceAll(expr, "+ -", "- ")

	if suffix != "" {
		if utf8.RuneCountInString(suffix) == 1 {
			expr += ` + "` + suffix + `"`
		}
	}
	return expr
}
