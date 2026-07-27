package testutil

import (
	"errors"
	"runtime"
	"testing"
)

type fatalTracker struct {
	failed bool
	msg    string
}

func (f *fatalTracker) Helper() {}
func (f *fatalTracker) Fatal(args ...any) {
	f.failed = true
	runtime.Goexit()
}
func (f *fatalTracker) Fatalf(format string, args ...any) {
	f.failed = true
	f.msg = format
	runtime.Goexit()
}

func TestAssertEqual(t *testing.T) {
	AssertEqual(t, 1, 1)
	AssertEqual(t, "a", "a")
	var ft fatalTracker
	done := make(chan struct{})
	go func() {
		defer close(done)
		AssertEqual(&ft, 1, 2)
	}()
	<-done
	if !ft.failed {
		t.Fatal("expected fail")
	}
}

func TestAssertNoError(t *testing.T) {
	AssertNoError(t, nil)
	var ft fatalTracker
	done := make(chan struct{})
	go func() {
		defer close(done)
		AssertNoError(&ft, errors.New("x"))
	}()
	<-done
	if !ft.failed {
		t.Fatal("expected fail")
	}
}

func TestAssertError(t *testing.T) {
	AssertError(t, errors.New("x"))
	var ft fatalTracker
	done := make(chan struct{})
	go func() {
		defer close(done)
		AssertError(&ft, nil)
	}()
	<-done
	if !ft.failed {
		t.Fatal("expected fail")
	}
}

func TestAssertTrue(t *testing.T) {
	AssertTrue(t, true, "ok")
	var ft fatalTracker
	done := make(chan struct{})
	go func() {
		defer close(done)
		AssertTrue(&ft, false, "nope")
	}()
	<-done
	if !ft.failed {
		t.Fatal("expected fail")
	}
}
