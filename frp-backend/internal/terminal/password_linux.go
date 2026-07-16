//go:build linux

package terminal

import (
	"os"

	"golang.org/x/sys/unix"
)

func IsTerminal(fd int) bool {
	_, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	return err == nil
}

func ReadPassword(fd int) ([]byte, error) {
	oldState, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return nil, err
	}
	newState := *oldState
	newState.Lflag &^= unix.ECHO
	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &newState); err != nil {
		return nil, err
	}
	defer unix.IoctlSetTermios(fd, unix.TCSETS, oldState)
	return readPasswordLine(os.NewFile(uintptr(fd), "stdin"))
}
