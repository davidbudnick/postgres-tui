package types

import "testing"

func TestScreenString(t *testing.T) {
	if got := ScreenConnections.String(); got != "Connections" {
		t.Fatalf("got %q", got)
	}
	if got := Screen(-1).String(); got != "Unknown" {
		t.Fatalf("got %q", got)
	}
	if got := ScreenBrowser.String(); got != "Browser" {
		t.Fatalf("got %q", got)
	}
}
