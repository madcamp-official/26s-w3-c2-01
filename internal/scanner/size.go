package scanner

import "os"

func logicalSize(info os.FileInfo) int64 {
	if info.IsDir() || info.Size() < 0 {
		return 0
	}
	return info.Size()
}
