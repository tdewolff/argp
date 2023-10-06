// +build windows

package argp

import "fmt"

func TerminalSize() (int, int, error) {
	return 0, 0, fmt.Errorf("not available")
}
