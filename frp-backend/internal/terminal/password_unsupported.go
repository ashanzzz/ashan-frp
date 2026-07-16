//go:build !windows && !linux

package terminal

import "errors"

func IsTerminal(int) bool {
	return false
}

func ReadPassword(int) ([]byte, error) {
	return nil, errors.New("secure terminal password input is unsupported on this platform")
}
