// Copyright (c) 2026 the go-widgets authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package application

import (
	"github.com/go-widgets/tray"
)

// Spec is the application's identity and its optional tray. It is the piece that
// separates an application from a bare window: a name and identifier the OS and
// the tray tooltip can show, a version, and — when the app wants a menu-bar /
// system-tray presence — an icon and a builder for its menu.
type Spec struct {
	Name       string            // e.g. "News Reader" — tray tooltip
	Identifier string            // reverse-DNS, e.g. "com.example.app"
	Version    string            // e.g. "1.2.0"
	Icon       []byte            // PNG icon bytes for the tray (optional)
	Tray       func() *tray.Menu // optional: builds the menu-bar menu
}

// runLoop is the low-level window loop [Run] drives. It is a package variable
// rather than a direct call to [run] for one reason: it is the seam a test
// substitutes so the Spec-level tray composition (build the menu, attach, defer
// Quit) can be exercised without opening a real OS window — the native
// gw.Open/win.Run path being a launch-verified boundary that unit tests do not
// reach. Production never reassigns it.
var runLoop = run

// Run opens the application's window and runs it to completion, putting up a
// menu-bar tray (when Spec.Tray is set) attached to the window's run loop, and
// invoking onReady once after the first frame is shown. It mirrors window.Run
// but owns the whole app lifecycle. (The .app bundle + Dock icon are a
// build-time concern handled by the packager, not here.)
//
// The tray is ATTACHED, not Run: attaching joins the platform's already-running
// main loop (the one win.Run drives for the window) rather than starting a
// second one, which is the only way a window and a tray can coexist in one
// process. tray.Attach must be called on the platform's main/UI thread and, per
// its contract, may block until the tray is torn down, so it is started on its
// own goroutine and stopped with Quit when the window returns — whichever way
// the window loop ends (a clean quit, or an error), the deferred Quit tears the
// tray down so no menu-bar icon is orphaned.
//
// A backend that cannot attach a tray (a headless build, or a platform without
// native tray support) makes tray.Attach return an error, which is deliberately
// swallowed: a missing tray must not stop the window from running. The window
// is the application; the tray is a convenience on top of it.
func Run(s Spec, cfg Config, h Handler, onReady func()) error {
	if s.Tray != nil {
		t := tray.New(s.Icon).SetTooltip(s.Name).SetMenu(s.Tray())
		// Attach joins the window's already-running loop; run it on its own
		// goroutine (it may block) and stop it when the window returns.
		go func() { _ = t.Attach() }()
		defer t.Quit()
	}
	return runLoop(cfg, h, onReady)
}
