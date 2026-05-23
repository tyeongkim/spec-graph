//go:build windows

package flock

import (
	"fmt"
	"os"
)

func Lock(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}

	unlock := func() {
		f.Close()
	}

	return unlock, nil
}
