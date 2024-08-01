//go:build linux
// +build linux

package commit

type LinuxMkdirAllProvider struct{}

func (p *LinuxMkdirAllProvider) MkdirAll(root, unsafePath string, mode os.FileMode) (string, error) {
	return "", securejoin.MkdirAll(root, unsafePath, mode)
}

func getMkdirAllProvider() MkdirAllProvider {
	return &LinuxMkdirAllProvider{}
}
