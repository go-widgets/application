package application

import (
	"errors"
	"fmt"
	"sync"

	"github.com/go-macos/objc"
)

var openURLLoad struct {
	once sync.Once
	err  error
}

// openURLNative opens u with [NSWorkspace openURL:], the call LaunchServices
// serves, through the same purego Objective-C bridge the window and tray use.
// No `open` child process is started, so it works from a sandboxed or
// PATH-less launch and cannot be steered by argument parsing.
func openURLNative(u string) error {
	openURLLoad.once.Do(func() {
		if err := objc.Load(objc.AppKit, objc.Foundation); err != nil {
			openURLLoad.err = fmt.Errorf("application: loading AppKit: %w", err)
		}
	})
	if openURLLoad.err != nil {
		return openURLLoad.err
	}
	var err error
	objc.AutoreleasePool(func() {
		nsurl := objc.ClassID("NSURL").Send(objc.Sel("URLWithString:"), objc.NSString(u))
		if nsurl == 0 {
			err = fmt.Errorf("%w: NSURL rejected %q", ErrURLInvalid, u)
			return
		}
		ws := objc.ClassID("NSWorkspace").Send(objc.Sel("sharedWorkspace"))
		if !objc.Send[bool](ws, objc.Sel("openURL:"), nsurl) {
			err = errors.New("application: NSWorkspace could not open the URL")
		}
	})
	return err
}
