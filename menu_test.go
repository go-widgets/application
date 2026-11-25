// Copyright (c) 2026 the go-widgets authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package application

import (
	"errors"
	"testing"

	"github.com/go-widgets/toolkit"
	"github.com/go-widgets/tray"
	gw "github.com/go-widgets/window"
)

// menuWindow is a window with a menu bar: it records what it was given, and runs
// during, from inside Run, the way a menu changed while the app runs is.
type menuWindow struct {
	*fakeWindow
	names  []string
	menus  []*tray.Menu
	fail   error
	during func()
}

func (m *menuWindow) SetMenuBar(name string, menu *tray.Menu) error {
	m.names = append(m.names, name)
	m.menus = append(m.menus, menu)
	return m.fail
}

func (m *menuWindow) Run(root toolkit.Widget) error {
	if m.during != nil {
		m.during()
	}
	return m.fakeWindow.Run(root)
}

func resetMenu(t *testing.T) {
	t.Helper()
	menuState.mu.Lock()
	menuState.menu, menuState.win, menuState.title, lastMenuErr = nil, nil, "", nil
	menuState.mu.Unlock()
	t.Cleanup(func() {
		menuState.mu.Lock()
		menuState.menu, menuState.win, menuState.title, lastMenuErr = nil, nil, "", nil
		menuState.mu.Unlock()
	})
}

func TestConfigMenuReachesTheWindow(t *testing.T) {
	resetMenu(t)
	m := tray.NewMenu().Add(tray.EditMenu())
	w := &menuWindow{fakeWindow: newFakeWindow(true)}
	withWindow(t, w, false)
	if err := run(Config{Title: "Probe", Menu: m}, &recorder{}, nil, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(w.menus) != 1 || w.menus[0] != m || w.names[0] != "Probe" {
		t.Fatalf("the window was given %v / %v, want the configured menu named Probe", w.menus, w.names)
	}
}

func TestSetMenuBeforeRunIsAppliedWhenTheWindowOpens(t *testing.T) {
	resetMenu(t)
	m := tray.NewMenu().Add(tray.SubMenu("File", tray.NewMenu()))
	if err := SetMenu(m); err != nil {
		t.Fatalf("SetMenu before Run = %v, want nil (kept for later)", err)
	}
	w := &menuWindow{fakeWindow: newFakeWindow(true)}
	withWindow(t, w, false)
	if err := run(Config{Title: "T"}, &recorder{}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if len(w.menus) != 1 || w.menus[0] != m {
		t.Fatalf("the window was given %v, want the menu set before Run", w.menus)
	}
}

func TestSetMenuWhileRunningGoesToTheWindowAndStopsAfter(t *testing.T) {
	resetMenu(t)
	first := tray.NewMenu().Add(tray.SubMenu("A", tray.NewMenu()))
	second := tray.NewMenu().Add(tray.SubMenu("B", tray.NewMenu()))
	w := &menuWindow{fakeWindow: newFakeWindow(true)}
	var during error
	w.during = func() { during = SetMenu(second) }
	withWindow(t, w, false)
	if err := run(Config{Title: "T", Menu: first}, &recorder{}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if during != nil {
		t.Fatalf("SetMenu while running = %v", during)
	}
	if len(w.menus) != 2 || w.menus[0] != first || w.menus[1] != second {
		t.Fatalf("the window was given %v, want first then second", w.menus)
	}
	// The window is gone: a later SetMenu keeps the menu for the next window and
	// does not touch the closed one.
	if err := SetMenu(first); err != nil {
		t.Fatalf("SetMenu after Run = %v", err)
	}
	if len(w.menus) != 2 {
		t.Error("a closed window was given a menu")
	}
}

func TestAWindowWithoutAMenuBarDoesNotStopRun(t *testing.T) {
	resetMenu(t)
	w := newFakeWindow(true) // no MenuBar capability
	withWindow(t, w, false)
	if err := run(Config{Menu: tray.NewMenu()}, &recorder{}, nil, nil); err != nil {
		t.Fatalf("run with a menu on a window without a menu bar = %v, want nil", err)
	}
	if MenuError() != nil {
		t.Errorf("ErrNotSupported was recorded as a menu error: %v", MenuError())
	}
	// While running, SetMenu says so.
	w2 := &menuWindow{fakeWindow: newFakeWindow(true)}
	var got error
	w2.fail = gw.ErrNotSupported
	w2.during = func() { got = SetMenu(tray.NewMenu()) }
	withWindow(t, w2, false)
	if err := run(Config{}, &recorder{}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(got, ErrNotSupported) {
		t.Errorf("SetMenu = %v, want ErrNotSupported", got)
	}
}

func TestAMenuBarFailureIsKeptNotFatal(t *testing.T) {
	resetMenu(t)
	boom := errors.New("boom")
	w := &menuWindow{fakeWindow: newFakeWindow(true), fail: boom}
	withWindow(t, w, false)
	if err := run(Config{Menu: tray.NewMenu()}, &recorder{}, nil, nil); err != nil {
		t.Fatalf("run = %v, want the window to open anyway", err)
	}
	if !errors.Is(MenuError(), boom) {
		t.Errorf("MenuError = %v, want boom", MenuError())
	}
}
