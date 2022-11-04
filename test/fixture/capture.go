package fixture

import (
	"io/ioutil"
	"os"
)

func CaptureStdout(callback func()) (string, error) {
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	callback()

	w.Close()

	data, err := ioutil.ReadAll(r)

	if err != nil {
		return "", err
	}
	return string(data), err
}

func CaptureStderr(callback func()) (string, error) {
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stderr = w
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	callback()
	w.Close()

	data, err := ioutil.ReadAll(r)

	if err != nil {
		return "", err
	}
	return string(data), err
}
