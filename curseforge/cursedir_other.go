//go:build !windows

package curseforge

import "errors"

// Stub version, so that getCurseDir exists
func getCurseDir() (string, error) {
	return "", errors.New("not compiled for windows")
}
