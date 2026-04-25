package torrent

import (
	"fmt"
	"syscall"
)

func freeSpaceGB(path string) (int, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, fmt.Errorf("statfs %s: %w", path, err)
	}
	free := uint64(st.Bavail) * uint64(st.Bsize)
	return int(free >> 30), nil
}
