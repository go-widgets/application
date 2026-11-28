# application — cross-platform application lifecycle for go-widgets

[![CI](https://github.com/go-widgets/application/actions/workflows/ci.yml/badge.svg)](https://github.com/go-widgets/application/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/go-widgets/application.svg)](https://pkg.go.dev/github.com/go-widgets/application)
[![License: BSD-3-Clause](https://img.shields.io/badge/License-BSD--3--Clause-blue.svg)](LICENSE)

The layer above [`go-widgets/window`](https://github.com/go-widgets/window):
where `window` presents a single surface, `application` owns the whole app
lifecycle. It opens the native OS window, drives its run loop, blits the
handler's own RGBA framebuffer into it, composes a system-tray / menu-bar icon
alongside the window (via [`go-widgets/tray`](https://github.com/go-widgets/tray)),
threads the live system appearance (dark/light, accent, system font) into the
handler, translates native input into the toolkit's event vocabulary, publishes
an accessibility tree, and fires a `ready` callback once the first frame is on
screen. No WebKit, no wasm, no HTTP — every widget is drawn by the framework and
the window is just a bitmap surface plus a native event source. Everything is
`CGO_ENABLED=0`.

Opening a real OS window and pumping its event loop is a launch-verified
boundary, so the native `Run` path is excluded from the coverage gate (like the
wasm glue elsewhere); the contract, the event translation, the present gating,
the appearance pump, the launch-height seam and the ready-after-first-frame
counter are all unit-tested to 100%.

| platform | native window (`go-widgets/window`) | native tray (`go-widgets/tray`) |
|----------|-------------------------------------|---------------------------------|
| darwin   | Cocoa / `NSWindow` (purego + the Obj-C runtime) | `NSStatusItem` + `NSMenu` |
| windows  | win32 (`golang.org/x/sys/windows` syscalls)     | `Shell_NotifyIcon` + `TrackPopupMenu` |
| linux    | X11 / Wayland (pure-Go wire protocols)          | `StatusNotifierItem` over DBus |

The tray is **attached** to the window's own loop (`Tray.Attach`) once the
first frame is up. A tray that cannot be attached is `Run`'s error, not a
silence: the window is closed and `Run` returns the attach error wrapped, so
`errors.Is(err, tray.ErrNoBackend)` says why. That needs a tray release whose
backend implements `Attach` on the platform: darwin always has; Linux and
Windows do from the tray release carrying go-widgets/tray#36, and before it
`Spec.Tray` there is now this error rather than no tray at all. An application that wants its window
whatever happens to the tray leaves `Spec.Tray` nil and runs one itself.

`Run` closes the window when its loop returns, and puts back the toolkit
clipboard it installed from it, so an application can call `Run` again to open
a fresh window -- a tray app's "Open" after the window was closed -- without the
old one staying on screen. (On X11 a closed window used to stay mapped, frozen,
until a garbage collection finalised its connection.)

## Usage

```go
package main

import (
	"github.com/go-widgets/application"
	"github.com/go-widgets/tray"
)

func main() {
	spec := application.Spec{
		Name:       "News Reader",
		Identifier: "com.example.reader",
		Version:    "1.0.0",
		Icon:       iconPNG, // []byte, optional
		Tray: func() *tray.Menu { // optional
			return tray.NewMenu().Add(
				tray.Item("Refresh", func() { app.Refresh() }),
				tray.Item("Quit", func() { app.Quit() }),
			)
		},
	}

	cfg := application.Config{Title: "News Reader", Width: 1200, Height: 800}

	// handler renders the app's own framebuffer and takes its input; onReady
	// fires once, after the first frame is visible.
	err := application.Run(spec, cfg, handler, func() {
		log.Println("first frame is on screen")
	})
	if err != nil {
		log.Fatal(err)
	}
}
```

`handler` implements `application.Handler` (Frame / Resize / MouseDown / MouseMove
/ MouseUp / Scroll / Key). It may additionally implement any of the optional
capability interfaces — `AppearanceSink`, `ShortcutSink`, `ModifiedClicker`,
`ModifiedKeyer`, `SecondaryClicker`, `ContextMenuHost`, `ClipboardController`,
`Accessible`, `NativeControlProvider` — and the run loop, which never wraps the
handler, will honour each one it finds.

## Accessibility of painted widgets

A handler that paints its own go-widgets widgets is, to a screen reader, one
unlabelled rectangle: its painted buttons are not in the macOS AX tree, the
AT-SPI tree or the UIA tree until it says they exist. It does so by implementing
`Accessible`, and `A11yElementsOf` builds the answer from the widget tree it
paints:

```go
func (v *View) A11yElements() []application.A11yElement {
	return application.A11yElementsOf(v.root) // role, label, value, bounds, disabled
}
```

Each element carries the role, label and value its widget reports, at the
widget's bounds (device pixels). The back-end maps them to its platform
(`NSAccessibilityElement`, AT-SPI, UIA), publishes only when the description
changed, and turns a platform "press" into a click at the element's centre
through the handler's own `MouseDown`/`MouseUp`. The result is capped at
`MaxA11yElements` (1000). Widgets with no `A11y` method contribute nothing.

A disabled widget (`A11yElement.Disabled`) is published as such: macOS
`AXEnabled` false, UI Automation `IsEnabled` false, AT-SPI without
ENABLED/SENSITIVE, so a screen reader announces it as unavailable instead of
offering to press it.

`Handler` has nowhere to put a modifier key: `MouseDown(x, y int)` carries no
Shift, and `Key`'s own contract says it has *"no room to say which modifiers were
held"*. `ModifiedClicker` and `ModifiedKeyer` are how a handler receives them, and
without one a Shift-click or a Shift-arrow cannot reach the widget underneath
whatever that widget supports.

That list is checked against the code by
`TestEveryCapabilityIsNamedInThePackageDoc`, which walks the package for type
assertions off a `Handler` and insists both this file and the package doc name
each one. It had drifted three capabilities behind before the check existed.

For a non-window host (a desktop shell, wasmdesk, a tab in something larger),
`Bind(handler, scale)` and `BindScaled(handler, scaleOf)` return a
`*toolkit.Surface` wired to the same handler without opening a window.

## A tray application: `Raise`, `Show`, `Hide` and `Config.HideOnClose`

A tray app's "Open" item must bring back the window that is already there, and
closing the window must not quit the app. Both are in the package:

```go
application.Run(spec, application.Config{Title: "App", Width: 480, Height: 640, HideOnClose: true}, h, nil)

// in the tray's "Open" callback, on any goroutine:
if err := application.Raise(); err != nil { /* ErrNoWindow, ErrNotSupported */ }
```

- `Raise()` shows the window if hidden, restores it if minimised, brings it to the
  front and asks for the focus. `Show()` only makes it visible; `Hide()` takes it
  off the screen without closing it (`Run` keeps running, the handler keeps its
  state).
- `Config.HideOnClose` makes the window's own close control hide it instead of
  ending `Run`; the app then quits through its tray's Quit. On macOS a click on
  the app's Dock icon while the window is hidden shows and raises it again, as
  standard apps do (window v0.111.0, `applicationShouldHandleReopen:`); Windows
  and Linux have no equivalent event for a hidden window.
- Never a silent no-op: `ErrNoWindow` (before `Run` opened a window or after it
  returned; wraps `window.ErrClosed`) and `ErrNotSupported` (the window package's
  own error) say what could not be done.

| back-end | Raise / Show / Hide | HideOnClose |
|---|---|---|
| macOS (Cocoa) | yes | yes |
| Windows (Win32) | yes | yes |
| X11 | yes | yes |
| Wayland | Show, Hide; Raise is `ErrNotSupported` (no xdg request) | yes |
| GTK, Android, wasm | `ErrNotSupported` | `Run` returns `ErrNotSupported` |

## A second window: `OpenWindow`

```go
// In a handler callback (the "+" button's), on the UI thread:
w, err := application.OpenWindow(application.WindowConfig{Title: "Add server", Width: 420, Height: 260}, formHandler)
if err != nil { /* ErrNoWindow, ErrNotSupported */ }
w.OnClose(func() { /* the user closed it, or w.Close() did */ })
// later, from any goroutine:
w.Raise()
w.Close()
```

`OpenWindow` opens a second top-level window on the loop `Run` already drives.
`formHandler` is a `Handler` of its own and gets what the main window's handler
gets: frames at the panel's resolution, input, native controls, accessibility,
damage, the system appearance and the clipboard. Closing the second window leaves
`Run` running; closing the main window closes it. `ErrNoWindow` before `Run` has
opened the main window or after it returned; `ErrNotSupported` on a back-end
without the capability: macOS, Windows and GTK4 (`GO_WIDGETS_GTK=1`) have it; X11,
Wayland, wasm and Android answer `ErrNotSupported`. Parent/modality is not
offered.

## Native dialogs: `OpenFile`, `SaveFile`, `Alert`, `Confirm`

```go
err := application.OpenFile(application.OpenOptions{
    Title: "Open", Multiple: true,
    Filters: []application.Filter{{Name: "Images", Extensions: []string{"png", "jpg"}}},
}, func(r application.OpenResult) { /* r.Paths, or r.Err == ErrCancelled */ })
// SaveFile(SaveOptions{Name: "report.txt"}, func(SaveResult)), Alert(AlertOptions{...}, func(AlertResult)),
// Confirm(message, detail, yes, no, func(yes bool, err error))
```

- Asynchronous and safe from any goroutine: the call returns at once, and the callback runs exactly once on its own goroutine (never the UI thread). When the call itself returns an error, the callback is never called.
- Errors: `ErrNoWindow` before `Run` has opened the window, `ErrNotSupported` on a back-end without native dialogs (X11, Wayland, wasm), `ErrInvalidDialog` for an input that is out of bounds, holds a NUL byte or is not UTF-8 (limits: `MaxFilters`, `MaxExtensions`, `MaxButtons`, `MaxStringLen`, `MaxMessageLen`), and `ErrCancelled` in a result when the user dismissed the dialog.
- Paths come back as the platform gives them, never resolved or followed.
- Native on macOS (NSOpenPanel, NSSavePanel, NSAlert), GTK 4.10+ (GtkFileDialog, GtkAlertDialog) and Windows (IFileDialog, TaskDialog).

## The menu bar: `Config.Menu` and `SetMenu`

```go
cfg := application.Config{
	Title: "Notes",
	Menu: tray.NewMenu().Add(
		tray.SubMenu("File", tray.NewMenu().Add(
			tray.KeyItem("Save", nil, "s", tray.ModCommand, save),
		)),
		tray.EditMenu(), // Undo, Redo, Cut, Copy, Paste, Select All
	),
}
err := application.Run(spec, cfg, handler, nil)
```

The menu is the `go-widgets/tray` model (labels, key equivalents, checkmarks,
separators, submenus, handlers), the same one a tray icon uses.
`application.SetMenu(m)` changes it at any time, from any goroutine, before or
while the application runs; the platform menu is rebuilt only when
`Menu.Signature()` changed.

- macOS: the menu bar at the top of the screen, with the standard application
  menu (About, Hide, Quit) first, named after `Config.Title`. Without
  `tray.EditMenu()` copy and paste do not work in native text fields.
- Windows: the window's menu bar; key equivalents are shown as `Ctrl+S`.
- GTK 4: a `GtkPopoverMenuBar` above the window's content; key equivalents
  are window-wide shortcuts that take precedence over the content.
- X11, Wayland, wasm, Android: no menu bar. `SetMenu` answers
  `ErrNotSupported` and `Run` carries on without one.

## Opening a URL in the browser: `OpenURL`

```go
if err := application.OpenURL("https://example.com/oauth/authorize?..."); err != nil {
	// errors.Is(err, application.ErrURLScheme / ErrURLInvalid / ErrOpenURLUnsupported)
}
```

`OpenURL(rawURL string) error` opens the URL in the user's default browser and
returns once the OS has accepted the request (it does not wait for the browser).
It is what an OAuth sign-in needs.

Only absolute `http` and `https` URLs with a host (and a port within 1-65535, if
any) are accepted, up to 8 KiB. Anything else is refused before the OS sees it:
`ErrURLScheme` for a `file:`, `smb:`, UNC, `javascript:` or custom-scheme URL (the
URLs an app opens often come from a server's document, and those would launch a
local file or a registered handler), `ErrURLInvalid` for a malformed one. The URL
is re-serialised from its parse, so what was checked is what is opened.

| OS | Mechanism |
|---|---|
| macOS | `NSWorkspace openURL:` through the objc bridge (no `open` process) |
| Windows | `ShellExecuteW`, verb `open` |
| Linux | the `org.freedesktop.portal.OpenURI` portal over D-Bus (works from a sandbox), falling back to `xdg-open` |
| other | `ErrOpenURLUnsupported` |

The URL check and the Linux chain are covered by tests (the portal call against a
fake portal on a real session bus); the real OS calls need a desktop and are the
launch-verified boundary, like the window loop. `OPENURL_LIVE=<url> go test -run
TestOpenURLLive .` opens one real browser tab to prove it on the machine at hand.

## License

BSD-3-Clause. See [LICENSE](LICENSE).
