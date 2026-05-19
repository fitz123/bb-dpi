package render

import (
	"strings"
)

// envsubst replaces `${VAR}` and `$VAR` occurrences in s with values
// from vars. Mirrors GNU envsubst's behavior for the allowlist of
// variables that scripts/render-config:180 passes:
//
//	envsubst '$HOME $UUID $TAILSCALE_AUTH_KEY $TAILSCALE_HOSTNAME $INTERNAL_DNS_1 $COMPANY_DOMAIN'
//
// Unknown variables are left as-is (matches envsubst's allowlist
// semantics — names not in the whitelist are NOT substituted, even
// if present in the environment).
//
// $VAR vs ${VAR}: both forms are recognized. $VAR matches as long as
// the next character is not a valid identifier char (alnum or '_').
// ${VAR} matches greedily inside the braces.
func envsubst(s string, vars map[string]string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		c := s[i]
		if c != '$' {
			b.WriteByte(c)
			i++
			continue
		}
		// '$' at end of string — literal.
		if i+1 >= len(s) {
			b.WriteByte('$')
			i++
			continue
		}
		next := s[i+1]
		if next == '{' {
			// Find matching '}'.
			end := strings.IndexByte(s[i+2:], '}')
			if end < 0 {
				// Unterminated — emit literal '$' and keep going.
				b.WriteByte('$')
				i++
				continue
			}
			name := s[i+2 : i+2+end]
			if v, ok := vars[name]; ok {
				b.WriteString(v)
			} else {
				b.WriteString(s[i : i+2+end+1])
			}
			i = i + 2 + end + 1
			continue
		}
		if !isIdentStart(next) {
			b.WriteByte('$')
			i++
			continue
		}
		// $VAR — read [A-Za-z_][A-Za-z0-9_]*
		j := i + 1
		for j < len(s) && isIdentChar(s[j]) {
			j++
		}
		name := s[i+1 : j]
		if v, ok := vars[name]; ok {
			b.WriteString(v)
		} else {
			b.WriteString(s[i:j])
		}
		i = j
	}
	return b.String()
}

func isIdentStart(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_'
}

func isIdentChar(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}
