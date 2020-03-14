package reposerver

import (
	"path"

	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/util/gpg"
)

// StartGPGWatcher watches a given directory for creation and deletion of files and syncs the GPG keyring
func StartGPGWatcher(sourcePath string) error {
	log.Infof("Starting GPG sync watcher on directory '%s'", sourcePath)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Create == fsnotify.Create || event.Op&fsnotify.Remove == fsnotify.Remove {
					if gpg.IsShortKeyID(path.Base(event.Name)) {
						log.Infof("Updating GPG keyring on filesystem event")
						added, removed, err := gpg.SyncKeyRingFromDirectory(sourcePath)
						if err != nil {
							log.Errorf("Could not sync keyring: %s", err.Error())
						} else {
							log.Infof("Result of sync operation: keys added: %d, keys removed: %d", len(added), len(removed))
						}
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Errorf("%v", err)
			}
		}
	}()

	err = watcher.Add(sourcePath)
	if err != nil {
		return err
	}
	<-done
	return nil
}
