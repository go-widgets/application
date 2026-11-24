package application

import (
	"os"
	"testing"
)

// TestOpenURLLive opens a real browser tab, so it only runs when asked:
//
//	OPENURL_LIVE=http://127.0.0.1:9/openurl-proof go test -run TestOpenURLLive -v .
//
// It is how the macOS (NSWorkspace) and Windows (ShellExecute) paths are proven
// on a real desktop, which a headless CI runner cannot do.
func TestOpenURLLive(t *testing.T) {
	u := os.Getenv("OPENURL_LIVE")
	if u == "" {
		t.Skip("set OPENURL_LIVE to a URL to open it for real")
	}
	if err := OpenURL(u); err != nil {
		t.Fatal(err)
	}
}
