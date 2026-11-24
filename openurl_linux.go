package application

import "errors"

// linuxOpeners are the two ways to open a URL on Linux, in order: the desktop
// portal, then xdg-open. They are fields so the order and the fallback are
// tested without a bus or a desktop.
type linuxOpeners struct {
	portal func(u string) error
	xdg    func(u string) error
}

// open tries the portal first -- it is the only route that works from a Flatpak
// or Snap sandbox and it asks the user's own browser choice -- and falls back to
// xdg-open when the portal is absent or refuses. If both fail the error carries
// both reasons.
func (o linuxOpeners) open(u string) error {
	perr := o.portal(u)
	if perr == nil {
		return nil
	}
	xerr := o.xdg(u)
	if xerr == nil {
		return nil
	}
	return errors.Join(perr, xerr)
}

func openURLNative(u string) error {
	return linuxOpeners{portal: openViaPortal, xdg: openViaXDG}.open(u)
}
