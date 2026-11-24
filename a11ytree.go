// Copyright (c) the go-widgets/application authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package application

import "github.com/go-widgets/toolkit"

// MaxA11yElements bounds the tree [A11yElementsOf] reports. A screen reader
// cannot use thousands of stops, and each element costs the platform bridge
// several round-trips to the accessibility server, so a runaway tree (a log with
// a row per line, say) is cut rather than allowed to stall the window. Elements
// beyond the bound are dropped from the END of the reading order.
const MaxA11yElements = 1000

// A11yElementsOf describes a toolkit widget tree for [Accessible].
//
// A Handler that paints with go-widgets widgets of its own, rather than handing
// the framework a tree to host, is a single opaque rectangle to the platform's
// accessibility layer: its painted buttons and labels are not in the macOS AX
// tree, the AT-SPI tree or the UIA tree, because nothing told the bridge they
// exist. The handler implements [Accessible] by returning the elements of the
// root it paints:
//
//	func (v *View) A11yElements() []application.A11yElement {
//		return application.A11yElementsOf(v.root)
//	}
//
// The walk is [toolkit.WalkA11y], so each element carries the role, label and
// value its widget reports, at the widget's bounds, which are already in the
// handler's device-pixel space. Widgets that report nothing (no A11y method, or
// RolePresentation) contribute no element. Pressing an element is replayed by
// the back-end as a click at its centre through the handler's own MouseDown and
// MouseUp, so every action a click has, a press has.
//
// Whether an unnamed or zero-area element is worth publishing is the back-end's
// decision, as it is for a hosted tree. The result is capped at
// [MaxA11yElements]; a nil root gives nil. Change-driven publishing is the
// back-end's too: it compares the description with the one it last published and
// republishes only what changed, so calling this every frame costs one walk and
// no platform calls.
//
// Like every [Accessible] implementation it must be called between frames, on
// the goroutine that mutates the widget tree.
func A11yElementsOf(root toolkit.Widget) []A11yElement {
	if root == nil {
		return nil
	}
	nodes := toolkit.WalkA11y(root)
	if len(nodes) > MaxA11yElements {
		nodes = nodes[:MaxA11yElements]
	}
	out := make([]A11yElement, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, A11yElement{
			Role:  string(n.Role),
			Name:  n.Name,
			Value: n.Value,
			X:     n.Rect.X, Y: n.Rect.Y, W: n.Rect.W, H: n.Rect.H,
		})
	}
	return out
}
