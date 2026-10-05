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

The tray is **attached** to the window's own loop (`tray.Attach`) once the
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

## License

BSD-3-Clause. See [LICENSE](LICENSE).
