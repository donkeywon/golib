package util

import (
	"os"
)

func FileExist(path string) bool {
	return PathExist(path, false)
}

func DirExist(path string) bool {
	return PathExist(path, true)
}

func PathExist(path string, isDir bool) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}

	if info.IsDir() && !isDir || !info.IsDir() && isDir {
		return false
	}

	return true
}
