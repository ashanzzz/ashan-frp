//go:build windows

package terminal

import (
	"os"

	"golang.org/x/sys/windows"
)

func IsTerminal(fd int) bool {
	var mode uint32
	return windows.GetConsoleMode(windows.Handle(fd), &mode) == nil
}

func ReadPassword(fd int) ([]byte, error) {
	handle := windows.Handle(fd)
	var oldMode uint32
	if err := windows.GetConsoleMode(handle, &oldMode); err != nil {
		return nil, err
	}
	newMode := oldMode &^ windows.ENABLE_ECHO_INPUT
	if err := windows.SetConsoleMode(handle, newMode); err != nil {
		return nil, err
	}
	defer windows.SetConsoleMode(handle, oldMode)
	return readPasswordLine(os.NewFile(uintptr(fd), "stdin"))
}
