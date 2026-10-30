//go:build !windows

package preferences

import "os"

func reparse(string) bool { return false }
func restrict(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		return os.Chmod(path, 0700)
	}
	return os.Chmod(path, 0600)
}
