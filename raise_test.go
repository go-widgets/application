// Copyright (c) 2026 the go-widgets authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package application

import (
	"errors"
	"sync"
	"testing"

	"github.com/go-widgets/toolkit"
	gw "github.com/go-widgets/window"
)

// visWindow is a window that carries the Visibility capability and records the
// calls, and whose Run blocks until a test calls release, so the window is "open"
// while the test raises it from another goroutine.
type visWindow struct {
	*fakeWindow
	mu    sync.Mutex
	calls []string
}

func (v *visWindow) note(s string) error {
	v.mu.Lock()
	v.calls = append(v.calls, s)
	v.mu.Unlock()
	return nil
}
func (v *visWindow) Show() error  { return v.note("show") }
func (v *visWindow) Hide() error  { return v.note("hide") }
func (v *visWindow) Raise() error { return v.note("raise") }

var _ gw.Visibility = (*visWindow)(nil)

func (v *visWindow) got() []string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return append([]string(nil), v.calls...)
}

// startRun runs the application on w in the background and returns once the
// window is open (Run is inside its loop), plus a function that ends it.
func startRun(t *testing.T, w gw.Backend, cfg Config, release func()) (done chan error) {
	t.Helper()
	opened := make(chan struct{})
	origOpen := openWindow
	t.Cleanup(func() { openWindow = origOpen })
	openWindow = func(c gw.Config) (gw.Backend, error) {
		close(opened)
		return w, nil
	}
	done = make(chan error, 1)
	go func() { done <- run(cfg, &recorder{}, nil, nil) }()
	<-opened
	// run installs the current window right after Open returns.
	for i := 0; i < 1000; i++ {
		if _, err := currentWindow(); err == nil {
			break
		}
		toolkitYield()
	}
	return done
}

func toolkitYield() { _ = toolkit.Rect{} }

func TestRaiseShowHideReachTheOpenWindowFromAnotherGoroutine(t *testing.T) {
	w := &visWindow{fakeWindow: newFakeWindow(false)}
	orig := closedByItsOwnLoop
	closedByItsOwnLoop = false
	t.Cleanup(func() { closedByItsOwnLoop = orig })
	done := startRun(t, w, Config{Title: "T"}, nil)

	var wg sync.WaitGroup
	for _, f := range []func() error{Raise, Show, Hide} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := f(); err != nil {
				t.Errorf("call: %v", err)
			}
		}()
	}
	wg.Wait()
	if n := len(w.got()); n != 3 {
		t.Fatalf("the window saw %v, want show, hide and raise once each", w.got())
	}

	_ = w.Close()
	if err := <-done; err != nil {
		t.Fatalf("run: %v", err)
	}
}

func TestRaiseAnswersErrNoWindowBeforeAndAfterRun(t *testing.T) {
	for _, f := range []func() error{Raise, Show, Hide} {
		if err := f(); !errors.Is(err, ErrNoWindow) || !errors.Is(err, gw.ErrClosed) {
			t.Fatalf("before Run: %v, want ErrNoWindow (wrapping window.ErrClosed)", err)
		}
	}
	w := &visWindow{fakeWindow: newFakeWindow(true)}
	withWindow(t, w, false)
	if err := run(Config{}, &recorder{}, nil, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if err := Raise(); !errors.Is(err, ErrNoWindow) {
		t.Fatalf("after Run returned: %v, want ErrNoWindow, never a call on the closed window", err)
	}
	if n := len(w.got()); n != 0 {
		t.Fatalf("a closed window was asked %v", w.got())
	}
}

// A back-end without the capability is a typed error, not a silent no-op.
func TestRaiseOnABackendWithoutVisibilityIsErrNotSupported(t *testing.T) {
	w := newFakeWindow(false)
	orig := closedByItsOwnLoop
	closedByItsOwnLoop = false
	t.Cleanup(func() { closedByItsOwnLoop = orig })
	done := startRun(t, w, Config{}, nil)
	for name, f := range map[string]func() error{"Raise": Raise, "Show": Show, "Hide": Hide} {
		if err := f(); !errors.Is(err, ErrNotSupported) || !errors.Is(err, gw.ErrNotSupported) {
			t.Errorf("%s on a back-end without Visibility: %v, want ErrNotSupported", name, err)
		}
	}
	_ = w.Close()
	<-done
}

// Config.HideOnClose reaches the window's own config; absent, it stays off.
func TestConfigHideOnCloseIsHandedToTheWindow(t *testing.T) {
	for _, want := range []bool{true, false} {
		w := newFakeWindow(true)
		origOpen, origClosed := openWindow, closedByItsOwnLoop
		t.Cleanup(func() { openWindow, closedByItsOwnLoop = origOpen, origClosed })
		var got gw.Config
		openWindow = func(c gw.Config) (gw.Backend, error) { got = c; return w, nil }
		closedByItsOwnLoop = false
		if err := run(Config{HideOnClose: want}, &recorder{}, nil, nil); err != nil {
			t.Fatalf("run: %v", err)
		}
		if got.HideOnClose != want {
			t.Errorf("Config.HideOnClose=%v reached the window as %v", want, got.HideOnClose)
		}
	}
}

// A back-end that refuses HideOnClose makes Run fail with the typed error.
func TestRunReturnsErrNotSupportedWhenTheBackendRefusesHideOnClose(t *testing.T) {
	origOpen := openWindow
	t.Cleanup(func() { openWindow = origOpen })
	openWindow = func(c gw.Config) (gw.Backend, error) {
		if c.HideOnClose {
			return nil, gw.ErrNotSupported
		}
		return newFakeWindow(true), nil
	}
	if err := run(Config{HideOnClose: true}, &recorder{}, nil, nil); !errors.Is(err, ErrNotSupported) {
		t.Fatalf("run: %v, want ErrNotSupported", err)
	}
}
