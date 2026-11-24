// Copyright (c) the go-widgets/application authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package application

import (
	"testing"

	"github.com/go-widgets/toolkit"
)

func TestA11yElementsOfNilRoot(t *testing.T) {
	if got := A11yElementsOf(nil); got != nil {
		t.Fatalf("nil root: %v", got)
	}
}

func TestA11yElementsOfReportsAPaintedButton(t *testing.T) {
	b := toolkit.NewButton("Settings", func() {})
	box := toolkit.NewVBox()
	box.AddFixed(b, 40)
	box.SetBounds(toolkit.Rect{X: 10, Y: 20, W: 200, H: 40})

	got := A11yElementsOf(box)
	var found *A11yElement
	for i := range got {
		if got[i].Name == "Settings" {
			found = &got[i]
		}
	}
	if found == nil {
		t.Fatalf("no Settings element in %v", got)
	}
	if found.Role != string(toolkit.RoleButton) {
		t.Errorf("role %q, want %q", found.Role, toolkit.RoleButton)
	}
	if found.X != 10 || found.Y != 20 || found.W != 200 || found.H != 40 {
		t.Errorf("rect %d,%d %dx%d", found.X, found.Y, found.W, found.H)
	}
}

func TestA11yElementsOfIsBounded(t *testing.T) {
	box := toolkit.NewVBox()
	for i := 0; i < MaxA11yElements+50; i++ {
		box.AddFixed(toolkit.NewLabel("row"), 10)
	}
	if got := len(A11yElementsOf(box)); got != MaxA11yElements {
		t.Fatalf("%d elements, want the bound %d", got, MaxA11yElements)
	}
}
