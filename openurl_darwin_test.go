package application

import (
	"errors"
	"sync"
	"testing"
)

// An unterminated IPv6 literal is something NSURL answers nil to (measured on
// macOS 27; the empty string is accepted there), so this reaches the "NSURL
// rejected" branch without ever opening a browser.
func TestOpenURLNativeRefusesWhatNSURLRejects(t *testing.T) {
	if err := openURLNative("http://["); !errors.Is(err, ErrURLInvalid) {
		t.Fatalf("openURLNative(\"http://[\") = %v, want ErrURLInvalid", err)
	}
}

func TestOpenURLNativeReportsAFrameworkThatWillNotLoad(t *testing.T) {
	boom := errors.New("no appkit")
	savedLoad := loadAppKit
	defer func() { loadAppKit, openURLLoad.once, openURLLoad.err = savedLoad, sync.Once{}, nil }()
	loadAppKit = func() error { return boom }
	openURLLoad.once, openURLLoad.err = sync.Once{}, nil
	if err := openURLNative("https://example.com"); !errors.Is(err, boom) {
		t.Fatalf("got %v, want the load error", err)
	}
}
