package fixture

// Fixture functions for tests related to files

import "io/ioutil"

// MustReadFile must read a file from given path. Panics if it can't.
func MustReadFile(path string) string {
	retBytes, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return string(retBytes)
}
