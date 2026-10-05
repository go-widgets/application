// Copyright (c) 2026 the go-widgets authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package application

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/go-widgets/toolkit"
	"github.com/go-widgets/tray"
	gw "github.com/go-widgets/window"
)

// fakeWindow is a window back-end with no display: Run blocks until it is
// closed (the way a window waits for its close button), unless returnAtOnce
// says the loop ends by itself, as a closed window's does.
type fakeWindow struct {
	returnAtOnce bool

	mu     sync.Mutex
	closes int
	shut   chan struct{}
}

func newFakeWindow(returnAtOnce bool) *fakeWindow {
	return &fakeWindow{returnAtOnce: returnAtOnce, shut: make(chan struct{})}
}

func (f *fakeWindow) Run(root toolkit.Widget) error {
	if f.returnAtOnce {
		return nil
	}
	<-f.shut
	return nil
}

func (f *fakeWindow) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closes == 0 {
		close(f.shut)
	}
	f.closes++
	return nil
}

func (f *fakeWindow) closed() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closes
}

func (f *fakeWindow) Size() (int, int) { return 100, 80 }
func (f *fakeWindow) String() string   { return "fake" }

// clipWindow also carries an OS clipboard, which run installs toolkit-wide.
type clipWindow struct {
	*fakeWindow
	text string
}

func (c *clipWindow) ClipboardText() string        { return c.text }
func (c *clipWindow) SetClipboardText(text string) { c.text = text }

var _ gw.Clipboard = (*clipWindow)(nil)

// withWindow makes run open w, on a platform whose back-end leaves the window
// to Close (every one but Cocoa; see closedByItsOwnLoop).
func withWindow(t *testing.T, w gw.Backend, closesItself bool) {
	t.Helper()
	origOpen, origClosed := openWindow, closedByItsOwnLoop
	t.Cleanup(func() { openWindow, closedByItsOwnLoop = origOpen, origClosed })
	openWindow = func(gw.Config) (gw.Backend, error) { return w, nil }
	closedByItsOwnLoop = closesItself
}

// TestRunClosesTheWindowWhenItsLoopReturns is go-widgets/application#27.
//
// A back-end's Run returns when its window is asked to close, and on X11 that
// does not release the window: Close does. run never called it, so a closed
// window stayed on the screen, frozen, until a garbage collection finalised the
// connection. claimward-vpn-app-linux closed it through the only reference run
// left behind, the toolkit clipboard.
func TestRunClosesTheWindowWhenItsLoopReturns(t *testing.T) {
	w := newFakeWindow(true)
	withWindow(t, w, false)
	if err := run(Config{Title: "T"}, &recorder{}, nil, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if n := w.closed(); n != 1 {
		t.Fatalf("the window was closed %d times after its loop returned, want 1", n)
	}
}

// The clipboard run made toolkit-wide reads through the window; once the window
// is gone it must not stay installed.
func TestRunPutsTheClipboardBack(t *testing.T) {
	before := toolkit.CurrentClipboard()
	w := &clipWindow{fakeWindow: newFakeWindow(true)}
	withWindow(t, w, false)
	if err := run(Config{}, &recorder{}, nil, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := toolkit.CurrentClipboard(); got != before {
		t.Fatalf("after run the toolkit clipboard is %T (%v), want the one from before (%T)", got, got, before)
	}
}

// On Cocoa the loop closes the window itself, and a Close sent after it would
// be delivered to the NEXT window's loop instead; see closedByItsOwnLoop.
func TestRunLeavesAWindowThatClosedItselfAlone(t *testing.T) {
	w := newFakeWindow(true)
	withWindow(t, w, true)
	if err := run(Config{}, &recorder{}, nil, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if n := w.closed(); n != 0 {
		t.Fatalf("a window whose loop closed it was closed %d more times", n)
	}
}

func TestRunReportsAWindowThatWouldNotOpen(t *testing.T) {
	origOpen := openWindow
	defer func() { openWindow = origOpen }()
	openWindow = func(gw.Config) (gw.Backend, error) { return nil, errSentinel }
	if err := run(Config{}, &recorder{}, nil, nil); !errors.Is(err, errSentinel) {
		t.Fatalf("run = %v, want the open error", err)
	}
}

// quit closes the window from outside, which is how Run ends a loop whose tray
// could not be attached; and a loop that ends first leaves quit unheard.
func TestRunQuitClosesTheWindow(t *testing.T) {
	w := newFakeWindow(false)
	withWindow(t, w, false)
	quit := make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- run(Config{}, &recorder{}, nil, quit) }()
	close(quit)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("closing quit did not end the window's loop")
	}

	w2 := newFakeWindow(true)
	withWindow(t, w2, false)
	if err := run(Config{}, &recorder{}, nil, make(chan struct{})); err != nil {
		t.Fatalf("run: %v", err)
	}
}

// TestRunFailsLoudlyWhenTheTrayCannotAttach is go-widgets/application#28.
//
// On Linux the tray backend did not implement Attach, Run swallowed the error,
// and Spec.Tray put up no tray with nothing to say so. A tray asked for and not
// given now closes the window and is Run's error.
func TestRunFailsLoudlyWhenTheTrayCannotAttach(t *testing.T) {
	origLoop, origAttach := runLoop, attachTray
	defer func() { runLoop, attachTray = origLoop, origAttach }()

	w := newFakeWindow(false)
	withWindow(t, w, false)
	// The real run, over a window that stays open until it is closed: the only
	// thing that can end it here is Run reacting to the failed attach. run fires
	// onReady after a second frame, which a fake window never draws, so the stub
	// fires it itself, the way the first frame going up would.
	runLoop = func(cfg Config, h Handler, onReady func(), quit <-chan struct{}) error {
		go onReady()
		return run(cfg, h, nil, quit)
	}
	attachTray = func(*tray.Tray) error { return tray.ErrNoBackend }

	done := make(chan error, 1)
	go func() {
		done <- Run(Spec{Name: "t", Tray: func() *tray.Menu { return tray.NewMenu() }}, Config{}, &recorder{}, nil)
	}()
	select {
	case err := <-done:
		if !errors.Is(err, tray.ErrNoBackend) {
			t.Fatalf("Run = %v, want the attach error (tray.ErrNoBackend) wrapped", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("a tray that could not attach left the window running and Run returning nothing")
	}
	if w.closed() == 0 {
		t.Error("Run reported the tray but left the window open")
	}
}
