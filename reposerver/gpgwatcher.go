package reposerver

import (
	"fmt"
	"path"
	"time"

	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v2/util/gpg"
)

const maxRecreateRetries = 5

// StartGPGWatcher watches a given directory for creation and deletion of files and syncs the GPG keyring
func StartGPGWatcher(sourcePath string) error {
	log.Infof("Starting GPG sync watcher on directory '%s'", sourcePath)
	forceSync := false
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create fsnotify Watcher: %w", err)
	}
	defer func(watcher *fsnotify.Watcher) {
		if err = watcher.Close(); err != nil {
			log.Errorf("Error closing watcher: %v", err)
		}
	}(watcher)

	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) {
					// In case our watched path is re-created (i.e. during e2e tests), we need to watch again
					// For more robustness, we retry re-creating the watcher up to maxRecreateRetries
					if event.Name == sourcePath && event.Has(fsnotify.Remove) {
						log.Warnf("Re-creating watcher on %s", sourcePath)
						attempt := 0
						for {
							err = watcher.Add(sourcePath)
							if err != nil {
								log.Errorf("Error re-creating watcher on %s: %v", sourcePath, err)
								if attempt < maxRecreateRetries {
									attempt += 1
									log.Infof("Retrying to re-create watcher, attempt %d of %d", attempt, maxRecreateRetries)
									time.Sleep(1 * time.Second)
									continue
								} else {
									log.Errorf("Maximum retries exceeded.")
									close(done)
									return
								}
							}
							break
						}
						// Force sync because we probably missed an event
						forceSync = true
					}
					if gpg.IsShortKeyID(path.Base(event.Name)) || forceSync {
						log.Infof("Updating GPG keyring on filesystem event")
						added, removed, err := gpg.SyncKeyRingFromDirectory(sourcePath)
						if err != nil {
							log.Errorf("Could not sync keyring: %s", err.Error())
						} else {
							log.Infof("Result of sync operation: keys added: %d, keys removed: %d", len(added), len(removed))
						}
						forceSync = false
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
		return fmt.Errorf("failed to add a new source to the watcher: %w", err)
	}
	<-done
	return fmt.Errorf("Abnormal termination of GPG watcher, refusing to continue.")
}
