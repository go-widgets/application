// Copyright (c) 2026 the go-widgets authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package application

import (
	"reflect"
	"testing"

	"github.com/go-widgets/toolkit"
)

// controllingHandler is a Handler that also supplies native controls, so a test
// can prove bind routes them onto the surface.
type controllingHandler struct {
	recorder
	controls []toolkit.NativeControl
}

func (c *controllingHandler) NativeControls() []toolkit.NativeControl { return c.controls }

// A handler that supplies native controls has its NativeControls wired onto
// Surface.Controls, so a back-end that hosts native controls can reconcile them.
func TestBindWiresNativeControlsForAProvider(t *testing.T) {
	h := &controllingHandler{controls: []toolkit.NativeControl{
		{Kind: toolkit.NativeSecureEntry, Key: "pw"},
	}}
	surf := bind(toolkit.NewSurface(nil), h, func() float64 { return 1 }, &appearancePump{}, nil)
	if surf.Controls == nil {
		t.Fatal("bind did not wire Surface.Controls for a NativeControlProvider handler")
	}
	if got := surf.Controls(); !reflect.DeepEqual(got, h.controls) {
		t.Fatalf("Surface.Controls() = %v, want the handler's %v", got, h.controls)
	}
}

// A handler that supplies no native controls leaves Surface.Controls unset.
func TestBindLeavesNativeControlsUnsetForPlainHandler(t *testing.T) {
	surf := bind(toolkit.NewSurface(nil), &recorder{}, func() float64 { return 1 }, &appearancePump{}, nil)
	if surf.Controls != nil {
		t.Fatal("a handler without NativeControls should leave Surface.Controls nil")
	}
}
