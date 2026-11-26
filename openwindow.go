// Copyright (c) 2026 the go-widgets authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package application

import (
	"sync"
	"time"

	"github.com/go-widgets/toolkit"
	gw "github.com/go-widgets/window"
)

// WindowConfig describes a secondary window opened by [OpenWindow].
type WindowConfig struct {
	Title         string
	Width, Height float64 // logical points; zero lets the platform choose
}

// Window is a secondary window opened by [OpenWindow]. Its methods are safe from
// any goroutine.
type Window struct {
	mu      sync.Mutex
	child   gw.Child
	gone    bool
	onClose func()
	stop    chan struct{} // ends the window's repaint clock
}

// OpenWindow opens a second top-level window showing h, on the loop [Run] is
// already driving. It is what a "+" button that opens an add-an-item form in its
// own window calls.
//
// h gets everything the main window's handler gets: frames at the panel's own
// resolution, input, native controls, accessibility, damage, the system
// appearance and the clipboard. h is a handler of its own, not the main one.
//
// It must be called on the UI thread -- from the handler's input callbacks, a
// menu action, or anything else the loop calls -- because the platform toolkit is
// touched directly.
//
// It answers [ErrNoWindow] when [Run] has not opened the main window or has
// returned, and [ErrNotSupported] when the platform's window back-end cannot open
// a second window (X11, Wayland, wasm and Android today; macOS, Windows and GTK4
// can). Closing the second window leaves [Run] running; closing the main window
// closes it.
func OpenWindow(cfg WindowConfig, h Handler) (*Window, error) {
	parent, err := currentWindow()
	if err != nil {
		return nil, err
	}

	w := &Window{stop: make(chan struct{})}
	// The window's own scale once it exists (it can move between displays); the
	// parent's until then, which is what the first frame is drawn at.
	scaleOf := func() float64 {
		w.mu.Lock()
		var src any = parent
		if w.child != nil {
			src = w.child
		}
		w.mu.Unlock()
		if s, ok := src.(gw.Scaler); ok {
			if v := s.RenderScale(); v > 0 {
				return v
			}
		}
		return 1
	}

	if c, ok := parent.(gw.Clipboard); ok {
		if cc, ok := h.(ClipboardController); ok {
			cc.SetSystemClipboard(c)
		}
	}
	ap := newAppearancePump(parent, h)
	surf := bind(toolkit.NewSurface(nil), h, scaleOf, ap, nil)
	ap.start()

	child, err := gw.OpenChild(parent, gw.Config{
		Title:       cfg.Title,
		Width:       int(cfg.Width),
		Height:      int(cfg.Height),
		RenderScale: gw.NativeScale,
	}, surf, w.closed)
	if err != nil {
		return nil, err
	}
	w.mu.Lock()
	w.child = child
	w.mu.Unlock()

	// The handler's Frame is this window's clock too, exactly as in the main
	// window: see run.
	if r, ok := child.(gw.Repainter); ok {
		np, gated := h.(presenter)
		go tick(r, np, gated, w.stop)
	}
	return w, nil
}

// tick repaints r at 60 Hz, gated by the handler when it can say whether a
// present is wanted; it is the main window's clock in run, once per window.
func tick(r gw.Repainter, np presenter, gated bool, stop <-chan struct{}) {
	t := time.NewTicker(time.Second / 60)
	defer t.Stop()
	var st presentState
	for {
		select {
		case <-t.C:
			if shouldRepaint(gated, np, &st) {
				r.Repaint()
			}
		case <-stop:
			return
		}
	}
}

// closed is the back-end's goodbye: the user closed the window, or Close did.
func (w *Window) closed() {
	w.mu.Lock()
	if w.gone {
		w.mu.Unlock()
		return
	}
	w.gone = true
	close(w.stop)
	f := w.onClose
	w.onClose = nil
	w.mu.Unlock()
	if f != nil {
		f()
	}
}

// Close closes the window. It is safe to call more than once and after the user
// closed it.
func (w *Window) Close() error {
	w.mu.Lock()
	c, gone := w.child, w.gone
	w.mu.Unlock()
	if gone || c == nil {
		return nil
	}
	return c.Close()
}

// Raise brings the window to the front and asks for the keyboard focus. It
// answers [ErrNoWindow] once the window is closed.
func (w *Window) Raise() error {
	w.mu.Lock()
	c, gone := w.child, w.gone
	w.mu.Unlock()
	if gone || c == nil {
		return ErrNoWindow
	}
	return c.Raise()
}

// OnClose sets f to be called once, on the UI thread, when the window goes away
// -- the user's close control or [Window.Close]. It replaces any earlier
// callback. When the window is already closed f is called at once.
func (w *Window) OnClose(f func()) {
	w.mu.Lock()
	if w.gone {
		w.mu.Unlock()
		if f != nil {
			f()
		}
		return
	}
	w.onClose = f
	w.mu.Unlock()
}
