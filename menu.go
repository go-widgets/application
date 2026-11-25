// Copyright (c) 2026 the go-widgets authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package application

import (
	"errors"
	"sync"

	"github.com/go-widgets/tray"
	gw "github.com/go-widgets/window"
)

// The application's menu bar, from the same [tray.Menu] model a tray icon uses.
//
// [Config.Menu] sets it when [Run] opens the window; [SetMenu] changes it at any
// time, from any goroutine, before or while the application runs. The platform
// menu is rebuilt only when the menu changed ([tray.Menu.Signature]); the click
// handlers are always the latest ones.
//
//   - macOS: the menu bar at the top of the screen, the standard application menu
//     (About, Hide, Quit) first, named after [Config.Title]. Add
//     [tray.EditMenu] so copy and paste work in text fields.
//   - Windows: the window's menu bar, key equivalents shown as accelerator text.
//   - X11, Wayland, GTK, wasm, Android: no menu bar; [SetMenu] answers
//     [ErrNotSupported], and [Run] carries on without one (a menu is an
//     addition, never a reason for the application not to start).

// menuState is the menu the application wants, and the window it goes to.
var menuState struct {
	mu    sync.Mutex
	menu  *tray.Menu
	title string
	win   gw.Backend
}

// SetMenu installs m as the application's menu bar, or removes it when m is nil.
// Called before [Run] it is applied when the window opens. It answers
// [ErrNotSupported] where the back-end has no menu bar.
func SetMenu(m *tray.Menu) error {
	menuState.mu.Lock()
	menuState.menu = m
	win, title := menuState.win, menuState.title
	menuState.mu.Unlock()
	if win == nil {
		return nil
	}
	return gw.SetMenuBar(win, title, m)
}

// attachMenu is run's half: it applies the menu the configuration or an earlier
// SetMenu asked for to the new window, and detaches it again with the returned
// function.
func attachMenu(win gw.Backend, cfg Config) func() {
	menuState.mu.Lock()
	if cfg.Menu != nil {
		menuState.menu = cfg.Menu
	}
	menuState.win, menuState.title = win, cfg.Title
	m := menuState.menu
	menuState.mu.Unlock()
	if m != nil {
		// Best effort: a back-end without a menu bar must not stop the application.
		if err := gw.SetMenuBar(win, cfg.Title, m); err != nil && !errors.Is(err, ErrNotSupported) {
			menuError(err)
		}
	}
	return func() {
		menuState.mu.Lock()
		menuState.win = nil
		menuState.mu.Unlock()
	}
}

// menuError reports a menu bar that failed for a reason other than being
// unsupported. The window is open and usable, so it is not fatal; it is kept for
// [MenuError].
func menuError(err error) {
	menuState.mu.Lock()
	lastMenuErr = err
	menuState.mu.Unlock()
}

var lastMenuErr error

// MenuError is the error the last attempt to install the menu bar at [Run]
// failed with, or nil. [ErrNotSupported] is not recorded: it is the normal answer
// on a platform without a menu bar.
func MenuError() error {
	menuState.mu.Lock()
	defer menuState.mu.Unlock()
	return lastMenuErr
}
