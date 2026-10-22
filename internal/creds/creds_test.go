package creds

import "testing"

func TestWikiKey(t *testing.T) {
	cases := map[string]string{
		"www.appropedia.org":  "APPROPEDIA",
		"appropedia.org":      "APPROPEDIA",
		"en.wikipedia.org":    "WIKIPEDIA",
		"wiki.my-lab.example": "MY_LAB",
		"localhost:8080":      "LOCALHOST",
		"127.0.0.1:8080":      "",
		"[::1]:8080":          "",
		"":                    "",
		"www.appropedia.org.": "APPROPEDIA",
		"WWW.Appropedia.ORG":  "APPROPEDIA",
	}
	for host, want := range cases {
		if got := WikiKey(host); got != want {
			t.Errorf("WikiKey(%q) = %q, want %q", host, got, want)
		}
	}
}

func env(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

func TestLookupPrefersPerWikiPair(t *testing.T) {
	c, ok := Lookup("www.appropedia.org", env(map[string]string{
		"WIKISYNC_USERNAME_APPROPEDIA": "me@bot",
		"WIKISYNC_PASSWORD_APPROPEDIA": "wikipw",
		"WIKISYNC_USERNAME":            "generic@bot",
		"WIKISYNC_PASSWORD":            "genericpw",
	}))
	if !ok || c.Username != "me@bot" || c.Password != "wikipw" || c.Source != "WIKISYNC_USERNAME_APPROPEDIA" {
		t.Fatalf("Lookup = %+v, %v", c, ok)
	}
}

func TestLookupFallsBackToGeneric(t *testing.T) {
	c, ok := Lookup("www.appropedia.org", env(map[string]string{
		"WIKISYNC_USERNAME": "generic@bot",
		"WIKISYNC_PASSWORD": "genericpw",
	}))
	if !ok || c.Username != "generic@bot" || c.Source != "WIKISYNC_USERNAME" {
		t.Fatalf("Lookup = %+v, %v", c, ok)
	}
}

// Half a per-wiki pair is never mixed with half a generic pair.
func TestLookupNeverMixesSources(t *testing.T) {
	c, ok := Lookup("www.appropedia.org", env(map[string]string{
		"WIKISYNC_USERNAME_APPROPEDIA": "me@bot", // password missing
		"WIKISYNC_USERNAME":            "generic@bot",
		"WIKISYNC_PASSWORD":            "genericpw",
	}))
	if !ok || c.Username != "generic@bot" || c.Password != "genericpw" {
		t.Fatalf("Lookup = %+v, %v; want the complete generic pair", c, ok)
	}
}

func TestLookupIPHostUsesGeneric(t *testing.T) {
	c, ok := Lookup("127.0.0.1:8080", env(map[string]string{
		"WIKISYNC_USERNAME": "Admin@wikisync",
		"WIKISYNC_PASSWORD": "pw",
	}))
	if !ok || c.Username != "Admin@wikisync" {
		t.Fatalf("Lookup = %+v, %v", c, ok)
	}
}

func TestLookupNothingSet(t *testing.T) {
	if _, ok := Lookup("www.appropedia.org", env(nil)); ok {
		t.Fatal("want ok=false with no credentials set")
	}
}
