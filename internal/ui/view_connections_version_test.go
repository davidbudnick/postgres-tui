package ui

import "testing"

func TestDisplayAppVersion(t *testing.T) {
	cases := map[string]string{
		"":                    "",
		"dev":                 "",
		"v0.0.2":              "0.0.2",
		"0.0.2":               "0.0.2",
		"v0.0.2-2-g3a8997a":   "0.0.2*",
		"v1.0.0-dirty":        "1.0.0*",
		"v1.0.0-1-gabc-dirty": "1.0.0*",
		"abc1234":             "dev",
		"g3a8997a":            "dev",
		// non-numeric suffix before -g (don't strip as commit count)
		"v1.0.0-rc1-gdeadbee": "1.0.0-rc1*",
		// empty / hash-only after describe strip
		"v-gabc": "dev",
		"-gabc":  "dev",
	}
	for in, want := range cases {
		if got := displayAppVersion(in); got != want {
			t.Fatalf("%q → %q want %q", in, got, want)
		}
	}
}
