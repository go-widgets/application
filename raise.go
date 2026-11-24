// Copyright (c) 2026 the go-widgets authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package application

import (
	"sync"

	gw "github.com/go-widgets/window"
)

// ErrNotSupported is what [Raise], [Show], [Hide] and [Run] (with
// [Config.HideOnClose]) answer when the platform's window back-end cannot do it:
// never a silent no-op. It is the window package's own error, so errors.Is finds
// it through either name. Wayland, for one, has no request to raise a window;
// see [github.com/go-widgets/window.Visibility] for the per-back-end table.
var ErrNotSupported = gw.ErrNotSupported

// ErrNoWindow is what [Raise], [Show] and [Hide] answer when no window is open:
// before [Run] has opened one, or after it has returned. It wraps
// [github.com/go-widgets/window.ErrClosed], so errors.Is finds either.
var ErrNoWindow = gw.ErrClosed

// current is the window the running application opened, for the package-level
// Raise/Show/Hide. One application has one window, so one slot is enough; run
// sets it once the window is open and clears it before the window is released.
var current struct {
	mu  sync.Mutex
	win gw.Backend
}

func setCurrent(w gw.Backend) {
	current.mu.Lock()
	current.win = w
	current.mu.Unlock()
}

func currentWindow() (gw.Backend, error) {
	current.mu.Lock()
	defer current.mu.Unlock()
	if current.win == nil {
		return nil, ErrNoWindow
	}
	return current.win, nil
}

// Raise brings the application's window to the front, showing it first when it
// is hidden and restoring it when it is minimised, and asks for the keyboard
// focus. It is what a tray's "Open" item calls, and it is safe from any
// goroutine -- the tray's callback runs on its own.
//
// It answers [ErrNoWindow] when no window is open, and [ErrNotSupported] when
// the back-end cannot raise (Wayland). The platform may still refuse the focus.
func Raise() error { return onWindow(gw.Raise) }

// Show makes a hidden window visible again, without asking for the focus. See
// [Raise] for the errors.
func Show() error { return onWindow(gw.Show) }

// Hide takes the window off the screen without closing it: [Run] keeps running
// and the handler keeps its state, so [Show] or [Raise] puts back exactly what
// was there. See [Raise] for the errors.
func Hide() error { return onWindow(gw.Hide) }

func onWindow(f func(gw.Backend) error) error {
	w, err := currentWindow()
	if err != nil {
		return err
	}
	return f(w)
}
