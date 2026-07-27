package service

import "testing"

type nopCloser struct{}

func (nopCloser) Close() error      { return nil }
func (nopCloser) Disconnect() error { return nil }

func TestNewContainer(t *testing.T) {
	c := NewContainer(nil, nil)
	if c == nil {
		t.Fatal("nil container")
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
}
