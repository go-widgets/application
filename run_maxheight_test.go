// Copyright (c) 2026 the go-widgets authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package application

import (
	"testing"

	gw "github.com/go-widgets/window"
)

// TestLaunchHeight covers the launch-height branches by substituting the
// visibleScreenHeight seam: a height the caller set is honoured whatever the
// screen, an unset one adopts a usable screen height, and an unavailable or
// non-positive screen leaves it unset.
func TestLaunchHeight(t *testing.T) {
	orig := visibleScreenHeight
	defer func() { visibleScreenHeight = orig }()

	visibleScreenHeight = func() (int, bool) { return 1200, true }
	if got := launchHeight(480); got != 480 {
		t.Errorf("height set: launchHeight(480) = %d, want 480 (honoured)", got)
	}
	if got := launchHeight(0); got != 1200 {
		t.Errorf("height unset, screen available: launchHeight(0) = %d, want 1200", got)
	}
	visibleScreenHeight = func() (int, bool) { return 0, false }
	if got := launchHeight(0); got != 0 {
		t.Errorf("height unset, no screen: launchHeight(0) = %d, want 0", got)
	}
	visibleScreenHeight = func() (int, bool) { return 0, true }
	if got := launchHeight(0); got != 0 {
		t.Errorf("height unset, non-positive screen height: launchHeight(0) = %d, want 0", got)
	}
}

// The content size the caller asked for reaches the window's own config: this
// is what Cocoa is opened with, so a Config of 800 by 480 is not screen-tall.
func TestConfigSizeIsHandedToTheWindow(t *testing.T) {
	origScreen, origOpen, origClosed := visibleScreenHeight, openWindow, closedByItsOwnLoop
	t.Cleanup(func() { visibleScreenHeight, openWindow, closedByItsOwnLoop = origScreen, origOpen, origClosed })
	visibleScreenHeight = func() (int, bool) { return 1200, true }
	var got gw.Config
	openWindow = func(c gw.Config) (gw.Backend, error) { got = c; return newFakeWindow(true), nil }
	closedByItsOwnLoop = false
	if err := run(Config{Width: 800, Height: 480}, &recorder{}, nil, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got.Width != 800 || got.Height != 480 {
		t.Errorf("Config 800x480 reached the window as %dx%d", got.Width, got.Height)
	}
}
