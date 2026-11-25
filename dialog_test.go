// Copyright (c) the go-widgets/application authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package application

import (
	"errors"
	"sync"
	"testing"

	gw "github.com/go-widgets/window"
)

// dlgWindow is a fake back-end with the Dialogs capability that answers each
// request at once with a canned result, recording what it was asked.
type dlgWindow struct {
	*fakeWindow
	mu   sync.Mutex
	open []OpenOptions
	save []SaveOptions
	alrt []AlertOptions
}

func (d *dlgWindow) OpenFile(o OpenOptions, done func(OpenResult)) error {
	d.mu.Lock()
	d.open = append(d.open, o)
	d.mu.Unlock()
	go done(OpenResult{Paths: []string{"/chosen/a", "/chosen/b"}})
	return nil
}

func (d *dlgWindow) SaveFile(o SaveOptions, done func(SaveResult)) error {
	d.mu.Lock()
	d.save = append(d.save, o)
	d.mu.Unlock()
	go done(SaveResult{Err: ErrCancelled})
	return nil
}

func (d *dlgWindow) Alert(o AlertOptions, done func(AlertResult)) error {
	d.mu.Lock()
	d.alrt = append(d.alrt, o)
	d.mu.Unlock()
	go done(AlertResult{Button: 1})
	return nil
}

func TestDialogsBeforeRunAreErrNoWindow(t *testing.T) {
	noop := func(OpenResult) {}
	if err := OpenFile(OpenOptions{}, noop); !errors.Is(err, ErrNoWindow) {
		t.Errorf("OpenFile: %v", err)
	}
	if err := SaveFile(SaveOptions{}, func(SaveResult) {}); !errors.Is(err, ErrNoWindow) {
		t.Errorf("SaveFile: %v", err)
	}
	if err := Alert(AlertOptions{Message: "m"}, func(AlertResult) {}); !errors.Is(err, ErrNoWindow) {
		t.Errorf("Alert: %v", err)
	}
	if err := Confirm("m", "", "Y", "N", func(bool, error) {}); !errors.Is(err, ErrNoWindow) {
		t.Errorf("Confirm: %v", err)
	}
}

func TestDialogsReachTheWindowAndReturnItsAnswer(t *testing.T) {
	w := &dlgWindow{fakeWindow: newFakeWindow(false)}
	closed := closedByItsOwnLoop
	closedByItsOwnLoop = false
	t.Cleanup(func() { closedByItsOwnLoop = closed })
	done := startRun(t, w, Config{}, nil)
	defer func() { _ = w.Close(); <-done }()

	got := make(chan OpenResult, 1)
	opts := OpenOptions{Title: "t", Multiple: true, Filters: []Filter{{Name: "Images", Extensions: []string{"png"}}}}
	if err := OpenFile(opts, func(r OpenResult) { got <- r }); err != nil {
		t.Fatal(err)
	}
	if r := <-got; r.Err != nil || len(r.Paths) != 2 || r.Paths[0] != "/chosen/a" {
		t.Errorf("OpenFile result = %+v", r)
	}
	if len(w.open) != 1 || w.open[0].Title != "t" || !w.open[0].Multiple {
		t.Errorf("window got %+v", w.open)
	}

	sv := make(chan SaveResult, 1)
	if err := SaveFile(SaveOptions{Name: "a.txt"}, func(r SaveResult) { sv <- r }); err != nil {
		t.Fatal(err)
	}
	if r := <-sv; !errors.Is(r.Err, ErrCancelled) {
		t.Errorf("SaveFile result = %+v", r)
	}

	cf := make(chan bool, 1)
	if err := Confirm("sure?", "", "Yes", "No", func(yes bool, err error) { cf <- yes && err == nil }); err != nil {
		t.Fatal(err)
	}
	if <-cf { // the fake presses button 1, "No"
		t.Error("Confirm reported yes for button 1")
	}
	if len(w.alrt) != 1 || len(w.alrt[0].Buttons) != 2 || w.alrt[0].Buttons[0] != "Yes" {
		t.Errorf("window got %+v", w.alrt)
	}
}

func TestDialogOptionsAreValidatedBeforeTheWindowSeesThem(t *testing.T) {
	w := &dlgWindow{fakeWindow: newFakeWindow(false)}
	closed := closedByItsOwnLoop
	closedByItsOwnLoop = false
	t.Cleanup(func() { closedByItsOwnLoop = closed })
	done := startRun(t, w, Config{}, nil)
	defer func() { _ = w.Close(); <-done }()

	if err := OpenFile(OpenOptions{Title: "a\x00b"}, func(OpenResult) {}); !errors.Is(err, ErrInvalidDialog) {
		t.Errorf("NUL title: %v", err)
	}
	if len(w.open) != 0 {
		t.Error("an invalid request reached the window")
	}
}

func TestDialogsOnABackendWithoutThemAreErrNotSupported(t *testing.T) {
	w := newFakeWindow(false)
	closed := closedByItsOwnLoop
	closedByItsOwnLoop = false
	t.Cleanup(func() { closedByItsOwnLoop = closed })
	done := startRun(t, w, Config{}, nil)
	defer func() { _ = w.Close(); <-done }()
	if err := OpenFile(OpenOptions{}, func(OpenResult) {}); !errors.Is(err, gw.ErrNotSupported) {
		t.Errorf("OpenFile: %v, want ErrNotSupported", err)
	}
}
