//go:build !darwin && !windows && !linux

package application

func openURLNative(string) error { return ErrOpenURLUnsupported }
