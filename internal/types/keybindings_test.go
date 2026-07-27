package types

import "testing"

func TestDefaultKeyBindings(t *testing.T) {
	kb := DefaultKeyBindings()
	if kb.Quit != "q" || kb.Help != "?" || kb.Query != ";" {
		t.Fatalf("unexpected defaults: %+v", kb)
	}
}
