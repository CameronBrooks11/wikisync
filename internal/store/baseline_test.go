package store

import "testing"

func TestBaselineRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := WriteBaseline(dir, 42, "hello\n"); err != nil {
		t.Fatal(err)
	}
	text, ok, err := ReadBaseline(dir, 42)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || text != "hello\n" {
		t.Fatalf("got %q ok=%v", text, ok)
	}
}

func TestReadMissingBaseline(t *testing.T) {
	_, ok, err := ReadBaseline(t.TempDir(), 99)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("want ok=false for a missing baseline")
	}
}
