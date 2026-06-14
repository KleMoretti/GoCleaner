//go:build !windows

package cleaner

import "os"

func replaceFile(src, dst string) error {
	return os.Rename(src, dst)
}
