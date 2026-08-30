// Copyright (c) 2026 the go-widgets authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package application

import (
	"errors"
	"testing"
	"time"

	"github.com/go-widgets/toolkit"
	"github.com/go-widgets/tray"
)

// TestOnReadyFiresOnceAfterTheFirstFrame is the whole reason run threads an
// onReady through bind: it must fire exactly once, and only after the FIRST
// frame's buffer has been blitted — which, because Frame returns the buffer the
// host is about to present, is the moment the SECOND Frame call is entered. So
// the first call must not fire it, the second must, and no later call fires it
// again.
func TestOnReadyFiresOnceAfterTheFirstFrame(t *testing.T) {
	h := &recorder{}
	var ready int
	surf := bind(toolkit.NewSurface(nil), h, func() float64 { return 1 }, &appearancePump{}, func() { ready++ })

	surf.Frame()
	if ready != 0 {
		t.Fatalf("onReady fired on the first frame (%d); the first buffer is not on screen yet", ready)
	}
	surf.Frame()
	if ready != 1 {
		t.Fatalf("onReady should fire once on the second frame, fired %d times", ready)
	}
	surf.Frame()
	surf.Frame()
	if ready != 1 {
		t.Fatalf("onReady fired again after the first frame: %d times total", ready)
	}
	if h.frames != 4 {
		t.Fatalf("the handler's Frame was called %d times, want 4 (bind must not swallow frames)", h.frames)
	}
}

// A nil onReady is the common case (Bind/BindScaled, or a Run caller that does
// not want the callback): bind must simply never call it and never count.
func TestBindWithoutOnReadyIsHarmless(t *testing.T) {
	surf := bind(toolkit.NewSurface(nil), &recorder{}, func() float64 { return 1 }, &appearancePump{}, nil)
	surf.Frame()
	surf.Frame() // would panic on a nil call if bind did not guard it
}

// bind must tell the handler its size in framebuffer pixels the first time it is
// asked for a frame, and again only when the size or scale changes — the resize
// units the whole translation layer exists to get right.
func TestBindResizesOnChangeOnly(t *testing.T) {
	h := &recorder{}
	scale := 2.0
	surf := bind(toolkit.NewSurface(nil), h, func() float64 { return scale }, &appearancePump{}, nil)

	surf.Frame()
	surf.Frame() // same size and scale: no second resize
	if len(h.resizes) != 1 {
		t.Fatalf("resizes = %v, want exactly one for an unchanged surface", h.resizes)
	}
	if h.resizes[0][2] != 2.0 {
		t.Fatalf("resize scale = %v, want the 2.0 the host reported", h.resizes[0][2])
	}
	scale = 3.0 // a density change (window dragged to another display)
	surf.Frame()
	if len(h.resizes) != 2 {
		t.Fatalf("a scale change must re-notify the handler; resizes = %v", h.resizes)
	}
}

// Bind and BindScaled are the exported hosting seams for a non-window host (a
// desktop shell, wasmdesk, a tab in something larger). They must produce a live
// surface wired to the handler, with a fixed scale in Bind's case and a polled
// one in BindScaled's — and, for an Accessible handler, an Elements closure that
// publishes its tree so a screen reader sees more than one opaque rectangle.
func TestBindAndBindScaledProduceALiveSurface(t *testing.T) {
	h := &recorder{elements: []A11yElement{{Role: "button", Name: "Go"}}}
	surf := Bind(h, 2.0)
	surf.Frame()
	if len(h.resizes) != 1 || h.resizes[0][2] != 2.0 {
		t.Fatalf("Bind did not resize at the fixed scale: %v", h.resizes)
	}
	if surf.Elements == nil {
		t.Fatal("Bind did not publish an accessibility tree for an Accessible handler")
	}
	if got := surf.Elements(); len(got) != 1 || got[0].Name != "Go" {
		t.Fatalf("Elements() = %v, want the handler's one element", got)
	}

	h2 := &recorder{}
	scale := 1.5
	s2 := BindScaled(h2, func() float64 { return scale })
	s2.Frame()
	if len(h2.resizes) != 1 || h2.resizes[0][2] != 1.5 {
		t.Fatalf("BindScaled did not resize at the polled scale: %v", h2.resizes)
	}
}

// bind wires the surface's OnInput to route, so an event delivered to the
// surface reaches the handler in its own vocabulary. This drives that seam end
// to end rather than calling route directly.
func TestBindRoutesSurfaceInputToTheHandler(t *testing.T) {
	h := &recorder{}
	surf := bind(toolkit.NewSurface(nil), h, func() float64 { return 1 }, &appearancePump{}, nil)
	if surf.OnInput == nil {
		t.Fatal("bind left Surface.OnInput unset")
	}
	surf.OnInput(toolkit.Event{Kind: toolkit.EventClick, X: 7, Y: 9})
	assertCalls(t, h, []string{"down:7,9"})
}

// errSentinel is what the stubbed loop returns so a test can prove Run passed
// its return value straight back.
var errSentinel = errors.New("stub loop returned")

// TestRunWithoutTraySkipsTrayComposition checks the Spec.Tray == nil branch:
// Run drives the low-level loop directly and returns its result, without
// building or attaching a tray.
func TestRunWithoutTraySkipsTrayComposition(t *testing.T) {
	orig := runLoop
	defer func() { runLoop = orig }()

	var gotCfg Config
	var gotReady bool
	runLoop = func(cfg Config, h Handler, onReady func()) error {
		gotCfg = cfg
		gotReady = onReady != nil
		return errSentinel
	}

	err := Run(Spec{Name: "no tray"}, Config{Title: "T"}, &recorder{}, func() {})
	if !errors.Is(err, errSentinel) {
		t.Fatalf("Run returned %v, want the loop's own error", err)
	}
	if gotCfg.Title != "T" {
		t.Fatalf("Run passed cfg %+v, want Title T threaded through", gotCfg)
	}
	if !gotReady {
		t.Fatal("Run dropped the onReady callback on the way to the loop")
	}
}

// TestRunWithTrayBuildsTheMenu checks the Spec.Tray != nil branch: Run consults
// the menu builder (so a tray is composed) and still drives the loop and returns
// its result. The default build links no native tray backend, so Attach is a
// harmless no-op — the branch under test is that the menu was BUILT and the loop
// still ran, not that a real menu-bar icon appeared.
func TestRunWithTrayBuildsTheMenu(t *testing.T) {
	origLoop, origAttach := runLoop, attachTray
	defer func() { runLoop, attachTray = origLoop, origAttach }()

	// The stub loop DRIVES onReady, standing in for the first frame going up, so
	// the tray-attach-on-ready path (the fix for the tray never appearing) runs.
	looped := false
	runLoop = func(cfg Config, h Handler, onReady func()) error {
		looped = true
		onReady()
		return errSentinel
	}
	attached := make(chan *tray.Tray, 1)
	attachTray = func(tr *tray.Tray) { attached <- tr }

	built := 0
	consumerReady := false
	spec := Spec{
		Name: "With Tray",
		Icon: []byte("PNGBYTES"),
		Tray: func() *tray.Menu {
			built++
			return tray.NewMenu()
		},
	}
	err := Run(spec, Config{}, &recorder{}, func() { consumerReady = true })
	if !errors.Is(err, errSentinel) {
		t.Fatalf("Run returned %v, want the loop's error even with a tray", err)
	}
	if built != 1 {
		t.Fatalf("the tray menu builder ran %d times, want exactly 1", built)
	}
	if !consumerReady {
		t.Fatal("Run's onReady wrapper dropped the consumer's callback")
	}
	select {
	case <-attached:
	case <-time.After(2 * time.Second):
		t.Fatal("the tray was never attached from onReady")
	}
	if !looped {
		t.Fatal("Run built a tray but never drove the window loop")
	}
}

// TestRunTrayAttachesWithoutConsumerOnReady covers the onReady==nil branch of the
// ready wrapper: a tray app whose caller passes no onReady still attaches its
// tray once the loop is up.
func TestRunTrayAttachesWithoutConsumerOnReady(t *testing.T) {
	origLoop, origAttach := runLoop, attachTray
	defer func() { runLoop, attachTray = origLoop, origAttach }()

	runLoop = func(_ Config, _ Handler, onReady func()) error {
		onReady()
		return errSentinel
	}
	attached := make(chan struct{}, 1)
	attachTray = func(*tray.Tray) { attached <- struct{}{} }

	err := Run(Spec{Name: "t", Tray: func() *tray.Menu { return tray.NewMenu() }}, Config{}, &recorder{}, nil)
	if !errors.Is(err, errSentinel) {
		t.Fatalf("Run returned %v, want the loop's error", err)
	}
	select {
	case <-attached:
	case <-time.After(2 * time.Second):
		t.Fatal("the tray was never attached with a nil consumer onReady")
	}
}

// TestAttachTrayDefaultIsHarmless exercises the real attachTray: with no native
// backend linked on the test runner, Attach fails and the error is swallowed —
// the point is that the default is callable and does not panic.
func TestAttachTrayDefaultIsHarmless(t *testing.T) {
	attachTray(tray.New(nil))
}
