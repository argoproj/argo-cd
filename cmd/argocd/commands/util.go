package commands

import (
	"io/ioutil"
	"log"
)

// readLocalFile reads a file from disk and returns its contents as a string.
func readLocalFile(path string) []byte {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	return data
}
