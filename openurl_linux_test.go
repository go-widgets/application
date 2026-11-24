package application

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

func TestLinuxOpenersOrderAndFallback(t *testing.T) {
	boomP, boomX := errors.New("no portal"), errors.New("no xdg-open")
	var calls []string
	mk := func(perr, xerr error) linuxOpeners {
		calls = nil
		return linuxOpeners{
			portal: func(u string) error { calls = append(calls, "portal:"+u); return perr },
			xdg:    func(u string) error { calls = append(calls, "xdg:"+u); return xerr },
		}
	}
	if err := mk(nil, boomX).open("u"); err != nil || len(calls) != 1 || calls[0] != "portal:u" {
		t.Fatalf("portal ok: err=%v calls=%v (xdg must not run)", err, calls)
	}
	if err := mk(boomP, nil).open("u"); err != nil || len(calls) != 2 || calls[1] != "xdg:u" {
		t.Fatalf("fallback: err=%v calls=%v", err, calls)
	}
	err := mk(boomP, boomX).open("u")
	if !errors.Is(err, boomP) || !errors.Is(err, boomX) {
		t.Fatalf("both failed: %v must carry both reasons", err)
	}
}

func TestOpenURLNativeIsTheLinuxChain(t *testing.T) {
	// No bus and no xdg-open in a minimal CI container: the real chain must fail
	// with an error, not hang or panic. (With a desktop it would open a browser,
	// so only the deliberately unreachable environment is exercised.)
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/nonexistent/bus")
	t.Setenv("PATH", t.TempDir())
	if err := openURLNative("https://example.com"); err == nil {
		t.Fatal("expected failure with no bus and no xdg-open")
	}
}

func TestOpenViaXDGRunsTheOpenerWithTheURLAlone(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "args")
	script := "#!/bin/sh\nprintf '%s|' \"$@\" > " + out + "\n"
	if err := os.WriteFile(filepath.Join(dir, "xdg-open"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	if err := openViaXDG("https://example.com/x?a=b&c=d"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		b, err := os.ReadFile(out)
		if err == nil && len(b) > 0 {
			if string(b) != "https://example.com/x?a=b&c=d|" {
				t.Fatalf("xdg-open argv = %q", b)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the opener never ran")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestOpenViaXDGMissingAndBroken(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if err := openViaXDG("https://example.com"); err == nil {
		t.Fatal("no xdg-open on PATH must be an error")
	}
	// Present but not executable: Start fails, and that is reported too.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "xdg-open"), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	if err := openViaXDG("https://example.com"); err == nil {
		t.Fatal("an unrunnable xdg-open must be an error")
	}
}

type fakePortal struct{ got chan [2]string }

func (p *fakePortal) OpenURI(parent, uri string, opts map[string]dbus.Variant) (dbus.ObjectPath, *dbus.Error) {
	p.got <- [2]string{parent, uri}
	return "/org/freedesktop/portal/desktop/request/x/y", nil
}

// TestOpenViaPortalOverARealBus needs a session bus (dbus-run-session) and is
// skipped where there is none. It exports a portal on that bus and checks the
// call's wire shape: (parent_window s, uri s, options a{sv}).
func TestOpenViaPortalOverARealBus(t *testing.T) {
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		t.Skip("no session bus")
	}
	srv, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Skip(err)
	}
	defer srv.Close()
	p := &fakePortal{got: make(chan [2]string, 1)}
	if err := srv.Export(p, portalPath, portalIface); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.RequestName(portalName, dbus.NameFlagDoNotQueue); err != nil {
		t.Fatal(err)
	}
	if err := openViaPortal("https://example.com/cb?x=1"); err != nil {
		t.Fatal(err)
	}
	if got := <-p.got; got != [2]string{"", "https://example.com/cb?x=1"} {
		t.Fatalf("portal received %q", got)
	}
}
