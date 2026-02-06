//go:build windows

package localconfig

import (
	"fmt"
	"os"
)

func getFilePermission(fi os.FileInfo) error {
	if fi.Mode().Perm() == 0666 || fi.Mode().Perm() == 0444 {
		return nil
	}
	return fmt.Errorf("config file has incorrect permission flags:%s."+
		"change the file permission either to 0444 or 0666.", fi.Mode().Perm().String())
}
