// Copyright (c) 2026 the go-widgets authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package application

import (
	"fmt"

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
// Quit, close the window when the attach fails) can be exercised without opening a real OS window — the native
// gw.Open/win.Run path being a launch-verified boundary that unit tests do not
// reach. Production never reassigns it.
var runLoop = run

// Run opens the application's window and runs it to completion, putting up a
// menu-bar tray (when Spec.Tray is set) attached to the window's run loop, and
// invoking onReady once after the first frame is shown. It mirrors window.Run
// but owns the whole app lifecycle. (The .app bundle + Dock icon are a
// build-time concern handled by the packager, not here.) The window is closed
// when Run returns, whichever way its loop ended.
//
// The tray is ATTACHED, not Run: attaching joins the platform's already-running
// main loop (the one win.Run drives for the window) rather than starting a
// second one, which is the only way a window and a tray can coexist in one
// process. It is attached from onReady, on its own goroutine, and stopped with
// Quit when the window returns -- whichever way the window loop ends (a clean
// quit, or an error), the deferred Quit tears the tray down so no menu-bar icon
// is orphaned.
//
// ⛔ A tray that cannot be attached is an ERROR, not a shrug
// (go-widgets/application#28). Run used to swallow it so that "a missing tray
// must not stop the window", and on Linux, where the tray backend did not
// implement Attach, that meant Spec.Tray put no tray up and nothing said so --
// in an application whose only way back to a hidden window was that tray. Now
// the window is closed and Run returns the attach error, wrapped (errors.Is
// finds tray.ErrNoBackend). A caller that wants a window whatever happens to
// the tray leaves Spec.Tray nil and runs one itself.
func Run(s Spec, cfg Config, h Handler, onReady func()) error {
	if s.Tray == nil {
		return runLoop(cfg, h, onReady, nil)
	}
	t := tray.New(s.Icon).SetTooltip(s.Name).SetMenu(s.Tray())
	defer t.Quit()
	// Attach the tray from onReady, NOT before the loop starts: attaching joins
	// the window's NSApplication and its running main loop, and BOTH must exist
	// first — the window backend creates the NSApplication when it opens, and the
	// loop begins pumping right after. onReady fires once the first frame is on
	// screen, by when both are true; attaching earlier (a bare `go t.Attach()`
	// ahead of runLoop) races the window's own AppKit setup and lands on nothing,
	// which is why the status item never appeared. It runs on its own goroutine
	// because onReady is called from inside the window's loop.
	quit := make(chan struct{})
	failed := make(chan error, 1)
	ready := func() {
		go func() {
			if err := attachTray(t); err != nil {
				failed <- err
				close(quit) // closes the window, which ends runLoop
			}
		}()
		if onReady != nil {
			onReady()
		}
	}
	err := runLoop(cfg, h, ready, quit)
	select {
	case terr := <-failed:
		return fmt.Errorf("application: Spec.Tray could not be attached: %w", terr)
	default:
		return err
	}
}

// attachTray joins the tray to the host's now-running loop. A package var so a
// test can substitute it for the real [tray.Tray.Attach], which does real
// platform work.
var attachTray = func(t *tray.Tray) error { return t.Attach() }
