package types

import "testing"

func TestLogWriter(t *testing.T) {
	w := NewLogWriter()
	if _, err := w.Write([]byte(`{"level":"INFO","msg":"hi"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(`{"level":"DEBUG","msg":"skip"}`)); err != nil {
		t.Fatal(err)
	}
	if w.Len() != 1 {
		t.Fatalf("len=%d", w.Len())
	}
	logs := w.GetLogs()
	if len(logs) != 1 {
		t.Fatalf("logs=%d", len(logs))
	}
}
