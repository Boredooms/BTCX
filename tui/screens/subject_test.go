package screens

import (
	"strings"
	"testing"
)

// subject_test.go is the accept/reject table for classifySubject (design §B.3,
// NIT-3), including the bech32 min-length/charset rows and the "never
// default-to-wallet" rejects.

func TestClassifySubjectTable(t *testing.T) {
	hex64 := strings.Repeat("a", 64)
	hex64Mixed := strings.Repeat("Ab12", 16) // 64 hex chars, mixed case

	cases := []struct {
		name     string
		in       string
		wantKind string
		wantOK   bool
	}{
		// rule 1: 64-char all-hex -> tx
		{"txid-lower", hex64, "tx", true},
		{"txid-mixed-case", hex64Mixed, "tx", true},
		{"hex-63", strings.Repeat("a", 63), "", false},
		{"hex-65", strings.Repeat("a", 65), "", false},
		{"hex-64-nonhex", strings.Repeat("g", 64), "", false},

		// rule 2: an IP (v4 or v6) -> ip
		{"ipv4", "203.0.113.9", "ip", true},
		{"ipv6", "2001:db8::1", "ip", true},
		{"ipv4-bad", "999.0.0.1", "", false},

		// rule 3a: base58 address (len 26..35, base58 charset) -> wallet
		{"base58-34", "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2", "wallet", true},
		{"base58-33", "3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy", "wallet", true},
		{"base58-too-short-25", "1BvBMSEYstWetqTFn5Au4m4GF", "", false},
		{"base58-too-long-36", "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2xx", "", false},
		{"base58-invalid-char-0", "1BvBMSEYstWetqTFn5Au4m4GFg7xJaN0N2", "", false},

		// rule 3b: bech32 (bc1/tb1 prefix, [14,90], bech32 charset) -> wallet
		{"bech32-bc1", "bc1qar0srrr7xfkvy5l643lydnw9re59gtzzwf5mdq", "wallet", true},
		{"bech32-tb1", "tb1qw508d6qejxtdg4y5r3zarvary0c5xw7kxpjzsx", "wallet", true},
		{"bech32-too-short-13", "bc1qar0srrr7", "", false},
		{"bech32-bad-charset-b", "bc1bbbbbbbbbbb", "", false},

		// never default-to-wallet: garbage rejects
		{"empty", "", "", false},
		{"whitespace", "   ", "", false},
		{"two-char", "ab", "", false},
		{"random", "hello-world!!", "", false},
		{"50-hex", strings.Repeat("a", 50), "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			kind, ok := classifySubject(c.in)
			if ok != c.wantOK || kind != c.wantKind {
				t.Fatalf("classifySubject(%q) = (%q,%v), want (%q,%v)",
					c.in, kind, ok, c.wantKind, c.wantOK)
			}
		})
	}
}
