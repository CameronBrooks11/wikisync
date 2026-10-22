// Package creds finds bot-password credentials in the environment, preferring
// per-wiki variable names so several wikis can be configured side by side.
package creds

import (
	"net"
	"strings"
)

// Credentials is a username/password pair and the variable it came from.
type Credentials struct {
	Username string
	Password string
	Source   string // the username variable's name, e.g. WIKISYNC_USERNAME_APPROPEDIA
}

// WikiKey derives the per-wiki suffix from a host: the label before the
// top-level domain, uppercased, with anything outside A-Z and 0-9 turned into
// "_". "www.appropedia.org" -> "APPROPEDIA". A bare IP address has no key.
// Hosts under a two-part suffix (e.g. example.co.uk) get the wrong label; use
// the generic variable names for those.
func WikiKey(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if host == "" || net.ParseIP(host) != nil {
		return ""
	}
	labels := strings.Split(strings.Trim(host, "."), ".")
	label := labels[0]
	if len(labels) >= 2 {
		label = labels[len(labels)-2]
	}
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r - 'a' + 'A'
		case (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
			return r
		default:
			return '_'
		}
	}, label)
}

// Lookup returns the credentials for host: WIKISYNC_USERNAME_<KEY> and
// WIKISYNC_PASSWORD_<KEY> when both are set, otherwise WIKISYNC_USERNAME and
// WIKISYNC_PASSWORD when both are set. The pair always comes from one source.
func Lookup(host string, getenv func(string) string) (Credentials, bool) {
	if key := WikiKey(host); key != "" {
		userVar := "WIKISYNC_USERNAME_" + key
		user, pass := getenv(userVar), getenv("WIKISYNC_PASSWORD_"+key)
		if user != "" && pass != "" {
			return Credentials{Username: user, Password: pass, Source: userVar}, true
		}
	}
	user, pass := getenv("WIKISYNC_USERNAME"), getenv("WIKISYNC_PASSWORD")
	if user != "" && pass != "" {
		return Credentials{Username: user, Password: pass, Source: "WIKISYNC_USERNAME"}, true
	}
	return Credentials{}, false
}

// Hint names the variables Lookup reads for host, for error messages.
func Hint(host string) string {
	if key := WikiKey(host); key != "" {
		return "WIKISYNC_USERNAME_" + key + " and WIKISYNC_PASSWORD_" + key +
			" (or WIKISYNC_USERNAME and WIKISYNC_PASSWORD)"
	}
	return "WIKISYNC_USERNAME and WIKISYNC_PASSWORD"
}
