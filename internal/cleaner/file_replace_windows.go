//go:build windows

package cleaner

import syswindows "golang.org/x/sys/windows"

func replaceFile(src, dst string) error {
	srcPtr, err := syswindows.UTF16PtrFromString(src)
	if err != nil {
		return err
	}
	dstPtr, err := syswindows.UTF16PtrFromString(dst)
	if err != nil {
		return err
	}
	return syswindows.MoveFileEx(srcPtr, dstPtr, syswindows.MOVEFILE_REPLACE_EXISTING|syswindows.MOVEFILE_WRITE_THROUGH)
}
