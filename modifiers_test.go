package application

import (
	"strings"
	"testing"

	"github.com/go-widgets/toolkit"
)

// modRecorder is a Handler that also takes modifiers, so a test can assert that
// they survive the translation. It embeds recorder, so a press or key that is
// NOT routed to the new capability still shows up in the same call log and the
// test can tell the two paths apart.
type modRecorder struct {
	recorder
}

func (m *modRecorder) ModifiedClick(x, y int, mods Modifiers) {
	m.calls = append(m.calls, "mdown:"+itoa(x)+","+itoa(y)+describe(mods))
}

func (m *modRecorder) ModifiedKey(name string, mods Modifiers) {
	m.calls = append(m.calls, "mkey:"+name+describe(mods))
}

func describe(m Modifiers) string {
	var b strings.Builder
	for _, c := range []struct {
		held bool
		name string
	}{{m.Ctrl, "ctrl"}, {m.Shift, "shift"}, {m.Alt, "alt"}, {m.Meta, "meta"}} {
		if c.held {
			b.WriteString(":" + c.name)
		}
	}
	return b.String()
}

// TestAModifiedClickCarriesItsModifiers.
//
// ⛔ This is the whole point. MouseDown(x, y int) has nowhere to put Shift or
// ⌘, so an app on this contract could not express a Shift-click or a ⌘-click to
// the widget underneath, whatever that widget supported -- toolkit's ListBox and
// Table both build a multi-row selection from exactly those gestures, and all of
// it was unreachable with a mouse.
func TestAModifiedClickCarriesItsModifiers(t *testing.T) {
	m := &modRecorder{}
	route(m, toolkit.Event{Kind: toolkit.EventClick, X: 3, Y: 4})
	route(m, toolkit.Event{Kind: toolkit.EventClick, X: 5, Y: 6, Shift: true})
	route(m, toolkit.Event{Kind: toolkit.EventClick, X: 7, Y: 8, Meta: true})
	route(m, toolkit.Event{Kind: toolkit.EventClick, X: 9, Y: 10,
		Ctrl: true, Shift: true, Alt: true, Meta: true})
	assertCalls(t, &m.recorder, []string{
		"mdown:3,4",
		"mdown:5,6:shift",
		"mdown:7,8:meta",
		"mdown:9,10:ctrl:shift:alt:meta",
	})
}

// TestAHandlerThatCannotTakeModifiersStillGetsItsPress: the capability is
// opt-in, and adding it must not change what an existing handler receives.
func TestAHandlerThatCannotTakeModifiersStillGetsItsPress(t *testing.T) {
	r := &recorder{}
	route(r, toolkit.Event{Kind: toolkit.EventClick, X: 3, Y: 4, Shift: true})
	assertCalls(t, r, []string{"down:3,4"})
}

// TestAModifiedKeyCarriesShift.
//
// ⛔ Shift is the one nothing else carried. A command chord already had a route
// (ShortcutSink) but a Shift-Arrow is not a command chord, so a handler could
// not tell it from a plain Arrow -- which is every "extend the selection with
// the keyboard" gesture, unreachable.
func TestAModifiedKeyCarriesShift(t *testing.T) {
	m := &modRecorder{}
	route(m, toolkit.Event{Kind: toolkit.EventKeyDown, Code: "ArrowDown"})
	route(m, toolkit.Event{Kind: toolkit.EventKeyDown, Code: "ArrowDown", Shift: true})
	route(m, toolkit.Event{Kind: toolkit.EventKeyDown, Code: "PageUp", Shift: true})
	assertCalls(t, &m.recorder, []string{
		"mkey:Down", "mkey:Down:shift", "mkey:PageUp:shift",
	})
}

// TestTheShortcutSinkKeepsPrecedenceOverModifiedKey: a command chord with a
// rune has gone to Shortcut since before this capability existed, and a handler
// implementing both must keep receiving it there.
func TestTheShortcutSinkKeepsPrecedenceOverModifiedKey(t *testing.T) {
	m := &modRecorder{}
	route(m, toolkit.Event{Kind: toolkit.EventKeyDown, Code: "a", Ctrl: true})
	// A NAMED key with a command modifier is not a shortcut, so it does reach
	// ModifiedKey -- with the modifier it used to lose.
	route(m, toolkit.Event{Kind: toolkit.EventKeyDown, Code: "ArrowDown", Meta: true})
	assertCalls(t, &m.recorder, []string{"shortcut:a:ctrl", "mkey:Down:meta"})
}

// TestAHandlerThatCannotTakeModifiedKeysStillGetsItsKey.
func TestAHandlerThatCannotTakeModifiedKeysStillGetsItsKey(t *testing.T) {
	r := &recorder{}
	route(r, toolkit.Event{Kind: toolkit.EventKeyDown, Code: "ArrowDown", Shift: true})
	assertCalls(t, r, []string{"key:Down:\x00"})
}

// TestAKeyThisApplicationHasNoNameForIsDroppedEitherWay: keyName returning ""
// means the key is not part of the handler's vocabulary, and the new capability
// must not smuggle it in.
func TestAKeyThisApplicationHasNoNameForIsDroppedEitherWay(t *testing.T) {
	m := &modRecorder{}
	route(m, toolkit.Event{Kind: toolkit.EventKeyDown, Code: "F13", Shift: true})
	assertCalls(t, &m.recorder, nil)
}
