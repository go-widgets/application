package application

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
)

// maxURLLength bounds what OpenURL hands to the OS. Windows' shell refuses
// anything past INTERNET_MAX_URL_LENGTH (2083) in practice and the others have
// limits of their own; 8 KiB is generous for a sign-in link and still bounded.
const maxURLLength = 8192

var (
	// ErrURLScheme is returned (wrapped, so test with errors.Is) when the URL's
	// scheme is not http or https. It is a security boundary: the URLs an app
	// opens often come from a server's document, and a file:, smb:, UNC or
	// custom-scheme URL would make the OS launch a local file or a registered
	// handler.
	ErrURLScheme = errors.New("application: only http and https URLs are opened")
	// ErrURLInvalid is returned (wrapped) for a URL that does not parse, has no
	// host, or is empty or longer than the bound.
	ErrURLInvalid = errors.New("application: not a valid absolute URL")
	// ErrOpenURLUnsupported is returned when the platform has no way to open a
	// URL (a target with neither macOS, Windows nor Linux).
	ErrOpenURLUnsupported = errors.New("application: this platform cannot open a URL")
)

// openURLFunc is the OS boundary: it is handed an already-validated URL. It is
// a variable so the tests (and an embedder's own tests) never launch a browser.
var openURLFunc = openURLNative

// OpenURL opens rawURL in the user's default web browser and returns once the
// OS has accepted the request (it does not wait for the browser).
//
// Only absolute http and https URLs with a host are accepted; any other scheme
// is [ErrURLScheme], a malformed one [ErrURLInvalid]. The URL is re-serialised
// from its parse before it reaches the OS, so what was checked is what is
// opened.
//
//   - macOS: NSWorkspace openURL: through the objc bridge (no `open` process).
//   - Windows: ShellExecuteW with the "open" verb.
//   - Linux: the org.freedesktop.portal.OpenURI portal over D-Bus (which also
//     works from a sandbox), falling back to xdg-open when there is no portal.
func OpenURL(rawURL string) error {
	u, err := checkURL(rawURL)
	if err != nil {
		return err
	}
	return openURLFunc(u)
}

// checkURL validates rawURL and returns its canonical form.
func checkURL(raw string) (string, error) {
	if raw == "" || len(raw) > maxURLLength {
		return "", fmt.Errorf("%w: empty or longer than %d bytes", ErrURLInvalid, maxURLLength)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrURLInvalid, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("%w (got scheme %q)", ErrURLScheme, u.Scheme)
	}
	if u.Hostname() == "" {
		return "", fmt.Errorf("%w: no host", ErrURLInvalid)
	}
	if p := u.Port(); p != "" {
		// url.Parse checks that a port is digits, not that it is a port.
		if n, err := strconv.Atoi(p); err != nil || n > 65535 {
			return "", fmt.Errorf("%w: port %q out of range", ErrURLInvalid, p)
		}
	}
	return u.String(), nil
}
