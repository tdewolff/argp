// +build !windows

package argp

import (
	"syscall"
	"unsafe"
)

func TerminalSize() (int, int, error) {
	data := struct {
		Row    uint16
		Col    uint16
		Xpixel uint16
		Ypixel uint16
	}{}
	if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, uintptr(syscall.Stdin), syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&data))); err != 0 {
		return 0, 0, err
	}
	return int(data.Row), int(data.Col), nil
}
