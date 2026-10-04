// Package application owns a go-widgets app's whole lifecycle: it opens the
// native OS window, runs its event loop, blits the handler's own RGBA
// framebuffer into it, composes a system-tray / menu-bar icon alongside it, and
// fires a "ready" callback once the first frame is on screen. It is the sibling
// of go-widgets/window: window presents a single surface, application is the
// layer above it that turns a handler plus an identity (name, icon, tray) into a
// running application. There is no WebKit, no wasm, no HTTP — every widget is
// drawn by the framework (toolkit.Surface) and the window is just a bitmap
// surface plus a native event source. Everything is CGO_ENABLED=0.
//
// Opening a real OS window and pumping its event loop is inherently a
// launch-verified boundary, so the [Run] path through go-widgets/window is
// excluded from the coverage gate (like the wasm glue elsewhere); this contract,
// the event translation, the present gating, the appearance pump, the
// launch-height seam and the ready-after-first-frame counter are all unit-tested.
//
// The optional capability interfaces (AppearanceSink, ShortcutSink,
// SecondaryClicker, ContextMenuHost, ClipboardController, Accessible) are the
// reason the run loop must never WRAP a Handler: a wrapper would satisfy the
// bare Handler interface and silently drop every capability the concrete handler
// also implements. Everything here threads the ORIGINAL handler through, and
// type-asserts for a capability at the point of use.
package application

import (
	"errors"
	"image/color"

	"github.com/go-widgets/toolkit"
)

// ErrUnsupported is returned by [Run] on a platform without a native back-end.
var ErrUnsupported = errors.New("application: no native window back-end on this platform")

// SystemAppearance carries look-and-feel harvested from the host UI so the
// renderer can match the live system look rather than a fixed palette.
type SystemAppearance struct {
	// Dark is the effective dark/light mode (macOS effectiveAppearance).
	Dark bool
	// Accent is the user's accent colour (macOS controlAccentColor); only
	// meaningful when HasAccent is set.
	Accent    color.RGBA
	HasAccent bool
	// FontTTF is the raw system font (e.g. macOS SFNS.ttf). Empty on a poll that
	// only refreshes colours, so the already-installed font is kept.
	FontTTF []byte
}

// AppearanceSink is an optional [Handler] capability. A back-end that can read
// the host appearance (currently the macOS Cocoa back-end) pushes it so the UI
// adopts the native dark/light mode, accent colour, and system font.
type AppearanceSink interface {
	SystemAppearance(SystemAppearance)
}

// ShortcutSink is an optional [Handler] capability: a key pressed with a
// command-style modifier (Ctrl/Cmd) arrives here as a shortcut, r is the base
// rune (modifiers stripped), ctrl/meta report which modifier was held. Back-ends
// route a modifier chord here instead of through Key (which drops the modified
// rune) so the app can act on real-browser-style shortcuts like Cmd+=.
type ShortcutSink interface {
	Shortcut(r rune, ctrl, meta bool)
}

// Modifiers says which modifier keys were held when an input event happened.
// Alt is the Option (⌥) / Alt key; Meta is Command (⌘) on macOS and the Super /
// Windows key elsewhere.
type Modifiers struct {
	Ctrl, Shift, Alt, Meta bool
}

// ModifiedClicker is an optional [Handler] capability: a press arrives here
// WITH the modifier keys that were held, instead of through [Handler.MouseDown],
// which has no room to carry them.
//
// ⛔ Without it a handler cannot express a Shift-click or a ⌘-click to the
// widget underneath, whatever that widget supports. Measured: go-widgets
// toolkit's ListBox and Table both build a multi-row selection from Ctrl/⌘ and
// Shift clicks, and an app on this contract could reach NONE of it -- it had
// nothing to put in the event's Ctrl and Shift fields, so every click arrived
// as a plain one and a multi-selection was unreachable with a mouse.
//
// A handler implementing it receives presses here and NOT through MouseDown;
// Handler still requires MouseDown, so leave it as the plain-press path or have
// one call the other.
type ModifiedClicker interface {
	ModifiedClick(x, y int, m Modifiers)
}

// ModifiedKeyer is an optional [Handler] capability: a key press arrives here
// with the modifiers that were held, instead of through [Handler.Key], whose
// contract says it has "no room to say which modifiers were held".
//
// ⛔ Shift is the one that matters and the one nothing else carried. A command
// chord (Ctrl/⌘) already had a route -- [ShortcutSink], which keeps precedence
// over this -- but a Shift-Arrow is not a command chord, so a handler could not
// tell it from a plain Arrow. That is every "extend the selection with the
// keyboard" gesture, unreachable.
//
// name is the same symbolic label Key receives, and a handler implementing this
// receives key presses here and not through Key.
type ModifiedKeyer interface {
	ModifiedKey(name string, m Modifiers)
}

// SecondaryClicker is an optional [Handler] capability: a secondary (right /
// two-finger / Control-click) press arrives here as the context-menu gesture,
// with the same device-pixel coordinates as a MouseDown. A handler that has no
// context menu simply does not implement it, and the press is ignored.
type SecondaryClicker interface {
	SecondaryClick(x, y int)
}

// ContextMenuHost is an optional [Handler] capability: while it reports a context
// menu open, the window feeds every input event to ContextMenuEvent instead of
// the usual MouseDown/Move/Scroll/Key path, so the menu is modal. The handler
// closes the menu itself (an item fired, a click landed outside, Escape) and then
// reports it inactive again.
type ContextMenuHost interface {
	ContextMenuActive() bool
	ContextMenuEvent(ev toolkit.Event)
}

// SystemClipboard is a host OS text clipboard (read + write). Its method set is
// deliberately identical to the toolkit's back-end-neutral Clipboard interface,
// so the app can install a back-end's SystemClipboard as the toolkit-wide
// clipboard directly — making every text widget's copy/cut/paste, and the app's
// copy actions, go through the real OS pasteboard.
type SystemClipboard interface {
	ClipboardText() string
	SetClipboardText(text string)
}

// ClipboardController is an optional [Handler] capability. A back-end that can
// reach the platform pasteboard installs its [SystemClipboard] here at startup
// (the mirror of [AppearanceSink]: this one flows a capability OUT to the
// handler). A back-end without clipboard support installs nothing, leaving the
// toolkit's default in-process clipboard in place (copy/paste still work
// within the app, just not across the OS).
type ClipboardController interface {
	SetSystemClipboard(SystemClipboard)
}

// Handler is the presenter's data source and input sink. The window calls Frame
// each tick (and after each event) and blits the returned buffer only when it
// reports changed. Input coordinates are device pixels (points × backing scale)
// with a top-left origin, matching the framebuffer.
type Handler interface {
	// Frame returns the current RGBA framebuffer (w*h*4 bytes) and whether it
	// changed since the last call (damage gate).
	Frame() (buf []byte, w, h int, changed bool)
	// Resize maps the new logical size to device pixels; scale is the backing
	// scale factor (device pixels per point).
	Resize(w, h int, scale float64)
	// MouseDown reports a left button press at device-pixel coordinates.
	MouseDown(x, y int)
	// MouseMove reports pointer motion at device-pixel coordinates. It fires
	// continuously during a left-button drag (back-ends need not emit idle
	// hovers) so the handler can drive interactions like a divider resize.
	MouseMove(x, y int)
	// MouseUp reports a left button release at device-pixel coordinates.
	MouseUp(x, y int)
	// Scroll reports a wheel delta in device pixels.
	Scroll(dy int)
	// Key reports a key press: name is a symbolic label for editing keys
	// ("Backspace"/"Escape"/"Enter"), r the rune for a printable character.
	Key(name string, r rune)
}

// A11yElement is one element the handler wants the platform's accessibility
// layer to expose: what it is, what it says, and where it sits.
//
// Rect is in DEVICE PIXELS with a top-left origin — the same space Frame's
// buffer and the MouseDown coordinates use — because that is the only space this
// package and its handler already agree on. Each back-end converts it to
// whatever its own accessibility API wants (macOS: screen points, y-up).
//
// Role is a neutral name, not a platform constant: this package presents pixels
// and must not drag a platform's vocabulary into the handler. The back-end maps
// it (see axRole).
type A11yElement struct {
	Role       string
	Name       string
	Value      string
	X, Y, W, H int
}

// Accessible is implemented by a Handler that can describe what it is showing.
// A back-end that supports an accessibility API asks for the description when
// the platform requests it; a handler that does not implement this simply
// presents pixels, as before.
//
// It is a separate, optional interface rather than a Handler method so that
// adding it breaks no existing back-end or handler.
type Accessible interface {
	// A11yElements returns the current elements in reading order. It is called
	// from the platform's accessibility client, which on macOS is the main
	// thread, at unpredictable times — the implementation must be safe to call
	// between frames.
	A11yElements() []A11yElement
}

// Config controls the window.
type Config struct {
	Title         string
	Width, Height float64
}
