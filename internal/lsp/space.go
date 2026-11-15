package lsp

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// FreeSpace is how many bytes are free on the disk of a folder, or of the nearest folder above it
// that exists, as a data folder is made only when its server first starts.
func FreeSpace(folder string) (uint64, error) {
	for {
		var disk syscall.Statfs_t
		err := syscall.Statfs(folder, &disk)
		if err == nil {
			return disk.Bavail * uint64(disk.Bsize), nil
		}
		parent := filepath.Dir(folder)
		if !os.IsNotExist(err) || parent == folder {
			return 0, err
		}
		folder = parent
	}
}

// FolderSize is how many bytes the files under a folder take; 0 when there is none.
func FolderSize(folder string) int64 {
	var size int64
	_ = filepath.WalkDir(folder, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		if info, err := entry.Info(); err == nil {
			size += info.Size()
		}
		return nil
	})
	return size
}

// IndexesSize is how many bytes a server's indexes take in its data folder.
func IndexesSize(dataFolder string, server Server) int64 {
	if server.Indexes == "" || dataFolder == "" {
		return 0
	}
	return FolderSize(filepath.Join(dataFolder, server.Indexes))
}
