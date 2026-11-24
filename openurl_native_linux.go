package application

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	portalName  = "org.freedesktop.portal.Desktop"
	portalPath  = "/org/freedesktop/portal/desktop"
	portalIface = "org.freedesktop.portal.OpenURI"

	portalTimeout = 10 * time.Second
)

// openViaPortal calls org.freedesktop.portal.OpenURI.OpenURI(parent_window,
// uri, options) on its own session-bus connection (closed again, so it never
// shares state with the tray's). The call returns once the portal has accepted
// the request; the browser's launch is the portal's business.
func openViaPortal(u string) error {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return fmt.Errorf("application: no session bus: %w", err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), portalTimeout)
	defer cancel()
	call := conn.Object(portalName, dbus.ObjectPath(portalPath)).
		CallWithContext(ctx, portalIface+".OpenURI", 0, "", u, map[string]dbus.Variant{})
	if call.Err != nil {
		return fmt.Errorf("application: OpenURI portal: %w", call.Err)
	}
	return nil
}

// openViaXDG starts xdg-open and reaps it in the background. The URL has been
// validated as http(s), so it cannot begin with "-" and become an option.
func openViaXDG(u string) error {
	path, err := exec.LookPath("xdg-open")
	if err != nil {
		return fmt.Errorf("application: xdg-open: %w", err)
	}
	cmd := exec.Command(path, u)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("application: xdg-open: %w", err)
	}
	go cmd.Wait()
	return nil
}
