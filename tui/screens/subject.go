package screens

import (
	"net"
	"strings"
)

// subject.go holds classifySubject, the ordered classifier the Dashboard
// open-subject input and the palette :open verb use to turn a free-typed id
// into a (kind, ok) pair before navigation (design §B.3, NIT-3). It uses bare
// net.ParseIP — permitted here because the tui/... transport guard forbids
// transport (net/http, crypto/tls), not the bare net package's string parsing.
//
// The classifier is deliberately conservative: it never defaults to "wallet".
// A string that matches no rule returns ("", false) so the caller shows an
// honest note instead of navigating to a screen that cannot resolve it. A
// structurally-valid-but-unknown id (right shape, no such subject in the case)
// still classifies and navigates; the downstream screen then shows its own
// honest dataError.

// ClassifySubject is the exported entry point the Root (palette :open and the
// dashboard open-subject input) uses; it delegates to the package-internal
// classifier so both the screens package and tui share one classification rule.
func ClassifySubject(s string) (kind string, ok bool) { return classifySubject(s) }

// classifySubject maps a trimmed id to its subject kind in a fixed order:
//
//	rule 1: a 64-char all-hex string is a transaction id ("tx").
//	rule 2: a value net.ParseIP accepts is an IP ("ip").
//	rule 3: a base58 address (charset [1-9A-HJ-NP-Za-km-z], length 26..35) OR a
//	        bech32 address (bc1/tb1 prefix, total length in [14,90], post-prefix
//	        runes in the bech32 charset [02-9ac-hj-np-z]) is a wallet ("wallet").
//
// Anything else returns ("", false). No default-to-wallet.
func classifySubject(s string) (kind string, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	// A short all-digit string is a block height (bounded so it can't be an
	// amount or a huge number). Bitcoin heights are well under 10 million.
	if isBlockHeight(s) {
		return "height", true
	}
	if len(s) == 64 && allHex(s) {
		// 64-hex is ambiguous: a txid OR a block hash. A real block hash carries
		// many leading zero hex digits (proof-of-work difficulty); a txid does
		// not. Treat >= 8 leading zeros as a block hash — in practice exact (no
		// txid has 8+ leading zero nibbles by chance), honest otherwise.
		if leadingZeroNibbles(s) >= 8 {
			return SubjectBlock, true
		}
		return "tx", true
	}
	if net.ParseIP(s) != nil {
		return "ip", true
	}
	if isBase58Address(s) || isBech32Address(s) {
		return "wallet", true
	}
	return "", false
}

// isBlockHeight reports whether s is a plausible Bitcoin block height: all
// digits, 1..7 chars (heights are < 10,000,000), no leading zero past "0".
func isBlockHeight(s string) bool {
	if len(s) < 1 || len(s) > 7 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	if len(s) > 1 && s[0] == '0' {
		return false // no leading-zero heights (avoids odd inputs)
	}
	return true
}

// leadingZeroNibbles counts leading '0' hex digits in s.
func leadingZeroNibbles(s string) int {
	n := 0
	for _, r := range s {
		if r != '0' {
			break
		}
		n++
	}
	return n
}

// allHex reports whether every rune in s is a hexadecimal digit.
func allHex(s string) bool {
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

// isBase58Address reports whether s is a plausible base58 (P2PKH/P2SH) address:
// the Bitcoin base58 charset (no 0, O, I, l) and a length in the usual 26..35
// range. This is a shape check, not a checksum validation — a right-shaped but
// unknown address still navigates so the detail screen can report it honestly.
func isBase58Address(s string) bool {
	if len(s) < 26 || len(s) > 35 {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune(base58Alphabet, r) {
			return false
		}
	}
	return true
}

// base58Alphabet is the Bitcoin base58 charset [1-9A-HJ-NP-Za-km-z].
const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// isBech32Address reports whether s is a plausible bech32/bech32m address: a
// bc1/tb1 human-readable prefix, a total length within the bech32 bounds
// [14,90], and post-prefix runes drawn from the bech32 data charset (NIT-3).
// The HRP match is case-insensitive on the prefix but the data part must be the
// lowercase bech32 charset (bech32 forbids mixed case).
func isBech32Address(s string) bool {
	if len(s) < 14 || len(s) > 90 {
		return false
	}
	lower := strings.ToLower(s)
	var rest string
	switch {
	case strings.HasPrefix(lower, "bc1"):
		rest = s[3:]
	case strings.HasPrefix(lower, "tb1"):
		rest = s[3:]
	default:
		return false
	}
	if rest == "" {
		return false
	}
	for _, r := range rest {
		if !strings.ContainsRune(bech32Charset, r) {
			return false
		}
	}
	return true
}

// bech32Charset is the bech32 data charset: all lowercase alphanumerics except
// '1', 'b', 'i', 'o' — i.e. [02-9ac-hj-np-z] (NIT-3).
const bech32Charset = "023456789acdefghjklmnpqrstuvwxyz"
