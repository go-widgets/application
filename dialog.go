// Copyright (c) the go-widgets/application authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package application

import (
	gw "github.com/go-widgets/window"
)

// The native dialogs of the platform -- open-file, save-file and message --
// for the application's window. They are the window package's [gw.Dialogs]
// capability, reached the way [Raise] reaches a window: through the window
// [Run] opened.
//
// Threading. Every call is asynchronous and safe from any goroutine, the UI
// goroutine and a tray callback included. It validates its options, queues the
// dialog on the platform's UI thread and returns at once; the callback is then
// called exactly once, on a goroutine of its own and never on the UI thread, so
// it may block or open the next dialog. When the call returns an error the
// callback is never called: [ErrNoWindow] before [Run] has opened a window,
// [ErrNotSupported] where the platform's back-end has no dialogs yet (X11,
// Wayland and the browser until they land), or [ErrInvalidDialog] for options
// outside the bounds -- at most [MaxFilters] filters, [MaxExtensions]
// extensions per filter, [MaxButtons] buttons, strings of at most
// [MaxStringLen] bytes (messages [MaxMessageLen]), no NUL, valid UTF-8.
//
// Paths come back exactly as the platform gave them: nothing is resolved,
// cleaned or checked, and no link is followed.
type (
	// Filter is a named set of extensions, without the dot.
	Filter = gw.Filter
	// OpenOptions configures [OpenFile].
	OpenOptions = gw.OpenOptions
	// OpenResult is what an [OpenFile] callback receives.
	OpenResult = gw.OpenResult
	// SaveOptions configures [SaveFile].
	SaveOptions = gw.SaveOptions
	// SaveResult is what a [SaveFile] callback receives.
	SaveResult = gw.SaveResult
	// AlertOptions configures [Alert].
	AlertOptions = gw.AlertOptions
	// AlertResult is what an [Alert] callback receives.
	AlertResult = gw.AlertResult
)

// The bounds and errors of the dialog options, from the window package.
const (
	MaxFilters    = gw.MaxFilters
	MaxExtensions = gw.MaxExtensions
	MaxButtons    = gw.MaxButtons
	MaxStringLen  = gw.MaxStringLen
	MaxMessageLen = gw.MaxMessageLen
)

var (
	// ErrCancelled is the Err of a dialog the user dismissed without choosing.
	ErrCancelled = gw.ErrCancelled
	// ErrInvalidDialog wraps every rejection of a dialog's options.
	ErrInvalidDialog = gw.ErrInvalidDialog
)

// OpenFile shows the platform's open-file dialog: one file, or with
// o.Multiple several, or with o.Directories a directory. done receives the
// chosen paths, or [ErrCancelled].
func OpenFile(o OpenOptions, done func(OpenResult)) error {
	w, err := currentWindow()
	if err != nil {
		return err
	}
	return gw.OpenFile(w, o, done)
}

// SaveFile shows the platform's save-file dialog with o.Name suggested. done
// receives the path chosen, which may not exist yet, or [ErrCancelled].
func SaveFile(o SaveOptions, done func(SaveResult)) error {
	w, err := currentWindow()
	if err != nil {
		return err
	}
	return gw.SaveFile(w, o, done)
}

// Alert shows the platform's message dialog with o.Buttons, the first being
// the default, and reports the index of the one pressed.
func Alert(o AlertOptions, done func(AlertResult)) error {
	w, err := currentWindow()
	if err != nil {
		return err
	}
	return gw.Alert(w, o, done)
}

// Confirm is [Alert] with two buttons: done receives true when yes, the first
// and default, was pressed.
func Confirm(message, detail, yes, no string, done func(yes bool, err error)) error {
	w, err := currentWindow()
	if err != nil {
		return err
	}
	return gw.Confirm(w, message, detail, yes, no, done)
}
