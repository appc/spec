// +build linux

package main

import (
	"fmt"
	"os"
)

func checkMountImpl(d string, readonly bool) error {
	mountinfoPath := fmt.Sprintf("/proc/self/mountinfo")
	mi, err := os.Open(mountinfoPath)
	if err != nil {
		return err
	}
	defer mi.Close()

	isMounted, ro, err := parseMountinfo(mi, d)
	if err != nil {
		return err
	}
	if !isMounted {
		return fmt.Errorf("%q is not a mount point", d)
	}

	if ro == readonly {
		return nil
	} else {
		return fmt.Errorf("%q mounted ro=%t, want %t", d, ro, readonly)
	}
}
