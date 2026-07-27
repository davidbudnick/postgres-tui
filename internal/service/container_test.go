package service

import (
	"errors"
	"testing"
)

type cfgClose struct {
	ConfigService
	err error
}

func (c cfgClose) Close() error { return c.err }

type pgClose struct {
	PGService
	err error
}

func (p pgClose) Disconnect() error { return p.err }

func TestNewContainer(t *testing.T) {
	c := NewContainer(nil, nil)
	if c == nil {
		t.Fatal("nil container")
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestContainerClose_ConfigError(t *testing.T) {
	want := errors.New("cfg")
	c := NewContainer(cfgClose{err: want}, nil)
	if err := c.Close(); err != want {
		t.Fatalf("got %v want %v", err, want)
	}
}

func TestContainerClose_PGError(t *testing.T) {
	want := errors.New("pg")
	c := NewContainer(nil, pgClose{err: want})
	if err := c.Close(); err != want {
		t.Fatalf("got %v want %v", err, want)
	}
}

func TestContainerClose_BothErrors_PrefersPG(t *testing.T) {
	cfgErr := errors.New("cfg")
	pgErr := errors.New("pg")
	c := NewContainer(cfgClose{err: cfgErr}, pgClose{err: pgErr})
	if err := c.Close(); err != pgErr {
		t.Fatalf("got %v want pg error", err)
	}
}
