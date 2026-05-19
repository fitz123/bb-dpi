package render

import "testing"

func TestEnvsubst(t *testing.T) {
	vars := map[string]string{
		"HOME":               "/Users/example",
		"UUID":               "00000000-0000-0000-0000-000000000000",
		"TAILSCALE_AUTH_KEY": "auth-key-deterministic-fake-test",
	}
	cases := []struct {
		in, want string
	}{
		// Basic ${VAR} expansion.
		{"prefix${HOME}suffix", "prefix/Users/examplesuffix"},
		// Basic $VAR expansion.
		{"path is $HOME/here", "path is /Users/example/here"},
		// $VAR followed by non-ident — stops at boundary.
		{"$HOME.local", "/Users/example.local"},
		// $VAR at end of string.
		{"trailing $HOME", "trailing /Users/example"},
		// Multiple expansions on one line.
		{"$HOME/.config/$UUID", "/Users/example/.config/00000000-0000-0000-0000-000000000000"},
		// Unknown variable: left untouched (allowlist semantics).
		{"unknown ${UNDEFINED_VAR} here", "unknown ${UNDEFINED_VAR} here"},
		{"unknown $UNDEFINED_VAR here", "unknown $UNDEFINED_VAR here"},
		// Literal dollar sign at end.
		{"price: $", "price: $"},
		// Unterminated ${ — treated as literal.
		{"unterm ${HOME", "unterm ${HOME"},
		// Dollar followed by digit: not an ident.
		{"$0 and $1 are args", "$0 and $1 are args"},
		// Dollar followed by punctuation: literal.
		{"$,$.$/", "$,$.$/"},
		// Empty input.
		{"", ""},
		// No substitution needed.
		{"plain text", "plain text"},
		// Quoted JSON-style with embedded $UUID.
		{`"user": "$UUID"`, `"user": "00000000-0000-0000-0000-000000000000"`},
		// Identifier with digit inside but not at start.
		{"$TAILSCALE_AUTH_KEY here", "auth-key-deterministic-fake-test here"},
	}
	for _, c := range cases {
		got := envsubst(c.in, vars)
		if got != c.want {
			t.Errorf("envsubst(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
