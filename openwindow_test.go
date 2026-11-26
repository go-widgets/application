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
	gw "github.com/go-widgets/window"
)

// childParent is a main window that can open a second one.
type childParent struct {
	*fakeWindow
	cfg     gw.Config
	root    toolkit.Widget
	onClose func()
	err     error
	child   *fakeChild
	scaled  bool
	scale   float64
}

func (p *childParent) OpenChild(cfg gw.Config, root toolkit.Widget, onClose func()) (gw.Child, error) {
	if p.err != nil {
		return nil, p.err
	}
	p.cfg, p.root, p.onClose = cfg, root, onClose
	p.child = &fakeChild{onClose: onClose}
	if p.scaled {
		return scaledChild{p.child, p.scale}, nil
	}
	return p.child, nil
}

// fakeChild records its calls; Close calls the back-end's goodbye, as a real one
// does. It is a Repainter, so the window's clock runs.
type fakeChild struct {
	mu       sync.Mutex
	onClose  func()
	raises   int
	closes   int
	repaints int
}

func (c *fakeChild) Close() error {
	c.mu.Lock()
	c.closes++
	c.mu.Unlock()
	c.onClose()
	return nil
}
func (c *fakeChild) Raise() error     { c.mu.Lock(); c.raises++; c.mu.Unlock(); return nil }
func (c *fakeChild) Size() (int, int) { return 5, 6 }
func (c *fakeChild) String() string   { return "fake child" }
func (c *fakeChild) Repaint()         { c.mu.Lock(); c.repaints++; c.mu.Unlock() }
func (c *fakeChild) painted() int     { c.mu.Lock(); defer c.mu.Unlock(); return c.repaints }

func withCurrent(t *testing.T, w gw.Backend) {
	t.Helper()
	setCurrent(w)
	t.Cleanup(func() { setCurrent(nil) })
}

func TestOpenWindowWithNoMainWindow(t *testing.T) {
	setCurrent(nil)
	if _, err := OpenWindow(WindowConfig{}, &plainHandler{}); !errors.Is(err, ErrNoWindow) {
		t.Fatalf("err = %v, want ErrNoWindow", err)
	}
}

func TestOpenWindowOnABackendWithoutTheCapability(t *testing.T) {
	withCurrent(t, newFakeWindow(true))
	if _, err := OpenWindow(WindowConfig{}, &plainHandler{}); !errors.Is(err, ErrNotSupported) {
		t.Fatalf("err = %v, want ErrNotSupported", err)
	}
}

func TestOpenWindowPassesTheConfigAndPropagatesAnError(t *testing.T) {
	p := &childParent{fakeWindow: newFakeWindow(true)}
	withCurrent(t, p)
	w, err := OpenWindow(WindowConfig{Title: "Add server", Width: 320, Height: 160}, &plainHandler{})
	if err != nil || w == nil {
		t.Fatalf("OpenWindow = %v, %v", w, err)
	}
	if p.cfg.Title != "Add server" || p.cfg.Width != 320 || p.cfg.Height != 160 || p.cfg.RenderScale != gw.NativeScale {
		t.Fatalf("config = %+v", p.cfg)
	}
	if p.root == nil {
		t.Fatal("no scene was handed to the window")
	}
	w.Close()

	boom := errors.New("boom")
	p2 := &childParent{fakeWindow: newFakeWindow(true), err: boom}
	withCurrent(t, p2)
	if w, err := OpenWindow(WindowConfig{}, &plainHandler{}); err != boom || w != nil {
		t.Fatalf("OpenWindow = %v, %v; want nil, the error", w, err)
	}
}

func TestWindowCloseRaiseAndOnClose(t *testing.T) {
	p := &childParent{fakeWindow: newFakeWindow(true)}
	withCurrent(t, p)
	w, err := OpenWindow(WindowConfig{}, &plainHandler{})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	w.OnClose(func() { calls++ })

	if err := w.Raise(); err != nil || p.child.raises != 1 {
		t.Fatalf("Raise = %v, raises %d", err, p.child.raises)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("OnClose called %d times, want 1", calls)
	}
	// After it is closed: Close is a no-op, Raise says there is no window, a
	// second goodbye changes nothing, and a late OnClose runs at once.
	if err := w.Close(); err != nil || p.child.closes != 1 {
		t.Fatalf("second Close = %v, closes %d", err, p.child.closes)
	}
	if err := w.Raise(); !errors.Is(err, ErrNoWindow) {
		t.Fatalf("Raise after close = %v, want ErrNoWindow", err)
	}
	late := 0
	w.OnClose(func() { late++ })
	if late != 1 {
		t.Fatalf("OnClose on a closed window ran %d times, want 1", late)
	}
	w.OnClose(nil) // must not panic
}

func TestWindowsClockRunsUntilItCloses(t *testing.T) {
	p := &childParent{fakeWindow: newFakeWindow(true)}
	withCurrent(t, p)
	w, err := OpenWindow(WindowConfig{}, &plainHandler{})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for p.child.painted() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if p.child.painted() == 0 {
		t.Fatal("the window's clock never repainted it")
	}
	w.Close()
	time.Sleep(50 * time.Millisecond) // let a tick in flight finish
	n := p.child.painted()
	time.Sleep(100 * time.Millisecond)
	if p.child.painted() != n {
		t.Fatal("the clock kept running after the window closed")
	}
}

// scaledChild is a child window that reports a render scale.
type scaledChild struct {
	*fakeChild
	s float64
}

func (c scaledChild) RenderScale() float64 { return c.s }

// scaleSeen opens a window whose child reports s (or no scale at all when
// scaled is false), runs one frame and returns the scale the handler was told.
func scaleSeen(t *testing.T, scaled bool, s float64) float64 {
	t.Helper()
	p := &childParent{fakeWindow: newFakeWindow(true), scaled: scaled, scale: s}
	withCurrent(t, p)
	h := &resizeHandler{}
	w, err := OpenWindow(WindowConfig{}, h)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	p.root.(*toolkit.Surface).Frame()
	return h.scale
}

type resizeHandler struct {
	plainHandler
	scale float64
}

func (h *resizeHandler) Resize(w, hh int, scale float64) { h.scale = scale }

func TestWindowScaleComesFromItsChild(t *testing.T) {
	if got := scaleSeen(t, true, 3); got != 3 {
		t.Fatalf("scale = %v, want the child's 3", got)
	}
	if got := scaleSeen(t, true, 0); got != 1 {
		t.Fatalf("scale = %v, want 1 for a child reporting 0", got)
	}
	if got := scaleSeen(t, false, 0); got != 1 {
		t.Fatalf("scale = %v, want 1 for a child without a scale", got)
	}
}

// clipParent is a main window that also carries an OS clipboard.
type clipParent struct {
	*childParent
	text string
}

func (c *clipParent) ClipboardText() string        { return c.text }
func (c *clipParent) SetClipboardText(text string) { c.text = text }

func TestOpenWindowGivesTheHandlerTheClipboard(t *testing.T) {
	p := &clipParent{childParent: &childParent{fakeWindow: newFakeWindow(true)}}
	withCurrent(t, p)
	r := &recorder{}
	w, err := OpenWindow(WindowConfig{}, r)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if r.clipboard == nil {
		t.Fatal("the handler was not given the system clipboard")
	}
}

func TestASecondGoodbyeDoesNotPanic(t *testing.T) {
	w := &Window{stop: make(chan struct{})}
	calls := 0
	w.OnClose(func() { calls++ })
	w.closed()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("a second goodbye panicked: %v", r)
		}
		if calls != 1 {
			t.Fatalf("a second goodbye called OnClose again (%d)", calls)
		}
	}()
	w.closed()
}

func TestWindowCloseBeforeTheChildExists(t *testing.T) {
	w := &Window{stop: make(chan struct{})}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Raise(); !errors.Is(err, ErrNoWindow) {
		t.Fatalf("Raise = %v", err)
	}
}
