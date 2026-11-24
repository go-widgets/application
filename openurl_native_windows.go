package application

import (
	"fmt"

	"golang.org/x/sys/windows"
)

const swShowNormal = 1

// openURLNative opens u with ShellExecuteW and the "open" verb, which hands an
// http(s) URL to the user's default browser. The verb is fixed and the file is
// the validated URL, so nothing the caller sent can name a program.
func openURLNative(u string) error {
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(u)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrURLInvalid, err)
	}
	if err := windows.ShellExecute(0, verb, file, nil, nil, swShowNormal); err != nil {
		return fmt.Errorf("application: ShellExecute: %w", err)
	}
	return nil
}
