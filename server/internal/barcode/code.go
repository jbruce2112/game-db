package barcode

import (
	"fmt"
	"strings"
	"unicode"
)

// Normalize strips non-digits and canonicalizes UPC-A / EAN-13.
func Normalize(raw string) (string, error) {
	var b strings.Builder
	for _, r := range raw {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	s := b.String()
	switch len(s) {
	case 8, 12, 13:
		if len(s) == 13 && s[0] == '0' {
			s = s[1:]
		}
		return s, nil
	case 14:
		s = strings.TrimLeft(s, "0")
		if len(s) < 12 {
			s = strings.Repeat("0", 12-len(s)) + s
		}
		if len(s) == 13 && s[0] == '0' {
			s = s[1:]
		}
		if len(s) == 12 || len(s) == 13 {
			return s, nil
		}
		return "", fmt.Errorf("unsupported barcode length")
	default:
		if s == "" {
			return "", fmt.Errorf("barcode is empty")
		}
		return "", fmt.Errorf("barcode must be 8, 12, or 13 digits")
	}
}

// Variants returns lookup keys for a canonical code (UPC-A and EAN-13 forms).
func Variants(canonical string) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(s string) {
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	add(canonical)
	if len(canonical) == 12 {
		add("0" + canonical)
	}
	if len(canonical) == 13 && canonical[0] == '0' {
		add(canonical[1:])
	}
	return out
}
