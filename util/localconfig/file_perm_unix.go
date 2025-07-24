//go:build !windows

package localconfig

import (
	"fmt"
	"os"
)

func getFilePermission(fi os.FileInfo) error {
	if fi.Mode().Perm() == 0o600 || fi.Mode().Perm() == 0o400 {
		return nil
	}
	return fmt.Errorf("config file has incorrect permission flags:%s."+
		"change the file permission either to 0400 or 0600.", fi.Mode().Perm().String())
}
