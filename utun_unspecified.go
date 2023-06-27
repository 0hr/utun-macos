//go:build !darwin

package tun_macos

import (
	"os"
)

func Open(name string) (*os.File, error) {
	panic("not implemented")
}
