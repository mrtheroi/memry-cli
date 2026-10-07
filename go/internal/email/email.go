// Package email validates email addresses like the PHP CLI.
package email

import (
	"regexp"
	"strings"
)

// Normalize trims and lowercases the email like PHP's trim() and
// strtolower(), reporting whether it is then a valid address for PHP's
// filter_var(..., FILTER_VALIDATE_EMAIL), which rejects bytes that are not
// ASCII.
func Normalize(address string) (string, bool) {
	address = lowerASCII(strings.Trim(address, " \t\n\r\x00\x0B"))
	return address, isValid(address)
}

// lowerASCII lowercases the ASCII letters only, like PHP 8's strtolower().
func lowerASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}

// The parts of PHP's FILTER_VALIDATE_EMAIL regular expression (without
// FILTER_FLAG_EMAIL_UNICODE, flags /iD) that RE2 can run. Its lookaheads
// are checked in Go: see isValid.
const (
	hex4 = `[a-f0-9]{1,4}`
	ipv4 = `(?:25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9]?[0-9])(?:\.(?:25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9]?[0-9])){3}`
	atom = `[\x21\x23-\x27\x2A\x2B\x2D\x2F-\x39\x3D\x3F\x5E-\x7E]+`
	quot = `\x22(?:[\x01-\x08\x0B\x0C\x0E-\x1F\x21\x23-\x5B\x5D-\x7F]|\x5C[\x00-\x7F])*\x22`
)

var (
	localPart  = regexp.MustCompile(`(?i)^(?:` + atom + `|` + quot + `)(?:\.(?:` + atom + `|` + quot + `))*$`)
	domainName = regexp.MustCompile(`(?i)^(?:(?:xn--)?[a-z0-9]+(?:-+[a-z0-9]+)*\.)+(?:[a-z][a-z0-9]*|xn--[a-z0-9]+)(?:-+[a-z0-9]+)*$`)

	// The IP literals inside the brackets, one alternative each.
	ipv6Full       = regexp.MustCompile(`(?i)^ipv6:` + hex4 + `(?::` + hex4 + `){7}$`)
	ipv6Compressed = regexp.MustCompile(`(?i)^ipv6:(?:` + hex4 + `(?::` + hex4 + `){0,5})?::(?:` + hex4 + `(?::` + hex4 + `){0,5})?$`)
	ipv4Only       = regexp.MustCompile(`^` + ipv4 + `$`)
	ipv6FullIPv4   = regexp.MustCompile(`(?i)^ipv6:` + hex4 + `(?::` + hex4 + `){5}:` + ipv4 + `$`)
	ipv6CompIPv4   = regexp.MustCompile(`(?i)^ipv6:(?:` + hex4 + `(?::` + hex4 + `){0,3})?::(?:` + hex4 + `(?::` + hex4 + `){0,3}:)?` + ipv4 + `$`)

	hexThenColonOrBracket = regexp.MustCompile(`(?i)[a-f0-9][:\]]`)
	hexThenColon          = regexp.MustCompile(`(?i)[a-f0-9]:`)
	longLabel             = regexp.MustCompile(`[^.]{64,}`)
)

// isValid reports whether PHP's FILTER_VALIDATE_EMAIL accepts address.
func isValid(address string) bool {
	if len(address) > 320 {
		return false
	}
	for i := 0; i < len(address); i++ {
		if address[i] >= 0x80 {
			return false
		}
	}
	for end, units := range maxUnits(address) {
		// (?!unit{255,}) and (?!unit{65,}@) at the start.
		if units >= 255 || (units >= 65 && end < len(address) && address[end] == '@') {
			return false
		}
	}
	// A quoted local part may hold an '@', so every split is tried.
	for at := 0; at < len(address); at++ {
		if address[at] == '@' && localPart.MatchString(address[:at]) && isDomain(address[at+1:]) {
			return true
		}
	}
	return false
}

// isDomain reports whether domain is a domain name or an IP literal in
// brackets, lookaheads included.
func isDomain(domain string) bool {
	if domainName.MatchString(domain) {
		// (?!.*[^.]{64,}): no label of 64 characters or more.
		return !longLabel.MatchString(domain)
	}
	if len(domain) < 2 || domain[0] != '[' || domain[len(domain)-1] != ']' {
		return false
	}
	literal := domain[1 : len(domain)-1]
	// The IPv6 lookaheads read the rest of the address from after "IPv6:":
	// the rest of the literal and its closing bracket.
	afterTag := ""
	if len(literal) >= len("ipv6:") {
		afterTag = literal[len("ipv6:"):] + "]"
	}
	switch {
	case ipv6Full.MatchString(literal), ipv4Only.MatchString(literal), ipv6FullIPv4.MatchString(literal):
		return true
	case ipv6Compressed.MatchString(literal):
		// (?!(?:.*[a-f0-9][:\]]){7,})
		return len(hexThenColonOrBracket.FindAllString(afterTag, -1)) < 7
	case ipv6CompIPv4.MatchString(literal):
		// (?!(?:.*[a-f0-9]:){5,})
		return len(hexThenColon.FindAllString(afterTag, -1)) < 5
	}
	return false
}

// maxUnits returns, for each length i, the most units of the lookaheads,
// (?:\x22?\x5C[\x00-\x7E]\x22?|\x22?[^\x5C\x22]\x22?), that address[:i]
// is exactly made of, or -1 when it is not made of them.
func maxUnits(address string) []int {
	best := make([]int, len(address)+1)
	for i := range best {
		best[i] = -1
	}
	best[0] = 0
	for i := range len(address) {
		if best[i] < 0 {
			continue
		}
		for _, start := range afterOptionalQuote(address, i) {
			end := -1
			switch {
			case start >= len(address):
			case address[start] == '\\':
				if start+1 < len(address) && address[start+1] <= 0x7E {
					end = start + 2
				}
			case address[start] != '"':
				end = start + 1
			}
			if end < 0 {
				continue
			}
			for _, after := range afterOptionalQuote(address, end) {
				best[after] = max(best[after], best[i]+1)
			}
		}
	}
	return best
}

// afterOptionalQuote returns the positions after an optional '"' at i.
func afterOptionalQuote(s string, i int) []int {
	if i < len(s) && s[i] == '"' {
		return []int{i, i + 1}
	}
	return []int{i}
}
