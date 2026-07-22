package kexpluginsdk_test

import (
	"strings"
	"testing"

	"github.com/roidmc/kex-utils/pkg/kexpluginsdk"
)

type fakeFactory struct {
	name string
}

func (f fakeFactory) Name() string { return f.name }

func TestRegistry_RegisterGetHas(t *testing.T) {
	reg := kexpluginsdk.NewRegistry[kexpluginsdk.Factory]()
	reg.Register(fakeFactory{name: "a"})
	reg.Register(fakeFactory{name: "b"})

	if reg.Len() != 2 {
		t.Fatalf("Len: got %d, want 2", reg.Len())
	}
	if !reg.Has("a") {
		t.Fatal("Has(a): want true")
	}
	if reg.Has("missing") {
		t.Fatal("Has(missing): want false")
	}
	f, ok := reg.Get("a")
	if !ok || f.Name() != "a" {
		t.Fatalf("Get(a): got %v,%v want a,true", f, ok)
	}
	if _, ok := reg.Get("missing"); ok {
		t.Fatal("Get(missing): want false")
	}
}

func TestRegistry_Range(t *testing.T) {
	reg := kexpluginsdk.NewRegistry[kexpluginsdk.Factory]()
	reg.Register(fakeFactory{name: "x"})
	reg.Register(fakeFactory{name: "y"})

	seen := map[string]bool{}
	reg.Range(func(name string, f kexpluginsdk.Factory) bool {
		seen[name] = true
		return true
	})
	if len(seen) != 2 || !seen["x"] || !seen["y"] {
		t.Fatalf("Range visited %v, want x and y", seen)
	}

	// Returning false must stop iteration early.
	count := 0
	reg.Range(func(name string, f kexpluginsdk.Factory) bool {
		count++
		return false
	})
	if count != 1 {
		t.Fatalf("Range with early stop: visited %d, want 1", count)
	}
}

func TestRegistry_DuplicatePanics(t *testing.T) {
	reg := kexpluginsdk.NewRegistry[kexpluginsdk.Factory]()
	reg.Register(fakeFactory{name: "dup"})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Register duplicate: want panic, got none")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "duplicate factory") {
			t.Fatalf("Register duplicate panic: got %v, want message containing %q", r, "duplicate factory")
		}
	}()

	reg.Register(fakeFactory{name: "dup"})
}
