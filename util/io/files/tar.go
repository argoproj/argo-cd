package files

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type tgz struct {
	srcPath    string
	exclusions []string
	tarWriter  *tar.Writer
}

// Tgz will iterate over all files found in srcPath compressing them with gzip
// and archiving with Tar. Will invoke every given writer while generating the tgz.
// This is useful to generate checksums. Will exclude files matching the exclusions
// list blob.
func Tgz(srcPath string, exclusions []string, writers ...io.Writer) error {
	if _, err := os.Stat(srcPath); err != nil {
		return fmt.Errorf("error inspecting srcPath %q: %w", srcPath, err)
	}

	mw := io.MultiWriter(writers...)

	gzw := gzip.NewWriter(mw)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	t := &tgz{
		srcPath:    srcPath,
		exclusions: exclusions,
		tarWriter:  tw,
	}
	return filepath.Walk(srcPath, t.tgzFile)
}

// Untgz will loop over the tar reader creating the file structure at dstPath.
// Callers must make sure dstPath is:
//   - a full path
//   - points to an empty directory or
//   - points to a non existing directory
func Untgz(dstPath string, r io.Reader) error {
	if !filepath.IsAbs(dstPath) {
		return fmt.Errorf("dstPath points to a relative path: %s", dstPath)
	}

	gzr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("error while iterating on tar reader: %w", err)
		}
		if header == nil || header.Name == "." {
			continue
		}

		target := filepath.Join(dstPath, header.Name)
		// Sanity check to protect against zip-slip
		if !Inbound(target, dstPath) {
			return fmt.Errorf("illegal filepath in archive: %s", target)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			err := os.MkdirAll(target, 0755)
			if err != nil {
				return fmt.Errorf("error creating nested folders: %w", err)
			}
		case tar.TypeSymlink:
			// Sanity check to protect against symlink exploit
			linkTarget := filepath.Join(filepath.Dir(target), header.Linkname)
			realPath, err := filepath.EvalSymlinks(linkTarget)
			if os.IsNotExist(err) {
				realPath = linkTarget
			} else if err != nil {
				return fmt.Errorf("error checking symlink realpath: %s", err)
			}
			if !Inbound(realPath, dstPath) {
				return fmt.Errorf("illegal filepath in symlink: %s", linkTarget)
			}
			err = os.Symlink(realPath, target)
			if err != nil {
				return fmt.Errorf("error creating symlink: %s", err)
			}
		case tar.TypeReg:
			err := os.MkdirAll(filepath.Dir(target), 0755)
			if err != nil {
				return fmt.Errorf("error creating nested folders: %w", err)
			}

			f, err := os.Create(target)
			if err != nil {
				return fmt.Errorf("error creating file %q: %w", target, err)
			}
			w := bufio.NewWriter(f)
			if _, err := io.Copy(w, tr); err != nil {
				f.Close()
				return fmt.Errorf("error writing tgz file: %w", err)
			}
			f.Close()
		}
	}
	return nil
}

// tgzFile is used as a filepath.WalkFunc implementing the logic to write
// the given file in the tgz.tarWriter applying the exclusion pattern defined
// in tgz.exclusions. Only regular files will be added in the tarball.
func (t *tgz) tgzFile(path string, fi os.FileInfo, err error) error {
	if err != nil {
		return fmt.Errorf("error walking in %q: %w", t.srcPath, err)
	}

	relativePath, err := RelativePath(path, t.srcPath)
	if err != nil {
		return fmt.Errorf("relative path error: %s", err)
	}

	for _, exclusionPattern := range t.exclusions {
		found, err := filepath.Match(exclusionPattern, relativePath)
		if err != nil {
			return fmt.Errorf("error verifying exclusion pattern %q: %w", exclusionPattern, err)
		}
		if found {
			if fi.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
	}

	if !supportedFileMode(fi) {
		return nil
	}

	link := ""
	if IsSymlink(fi) {
		link, err = os.Readlink(path)
		if err != nil {
			return fmt.Errorf("error getting link target: %s", err)
		}
	}

	header, err := tar.FileInfoHeader(fi, link)
	if err != nil {
		return fmt.Errorf("error creating a tar file header: %w", err)
	}

	// update the name to correctly reflect the desired destination when untaring
	header.Name = relativePath

	if err := t.tarWriter.WriteHeader(header); err != nil {
		return fmt.Errorf("error writing header: %w", err)
	}

	// Only regular files needs to have their content copied.
	// Directories and symlinks are header only.
	if fi.Mode().IsRegular() {
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("error opening file %q: %w", fi.Name(), err)
		}
		defer f.Close()

		if _, err := io.Copy(t.tarWriter, f); err != nil {
			return fmt.Errorf("error copying tgz file to writers: %w", err)
		}
	}

	return nil
}

// supportedFileMode will return true if the file mode is supported.
// Supported files means that it will be added to the tarball.
func supportedFileMode(fi os.FileInfo) bool {
	mode := fi.Mode()
	if mode.IsRegular() || mode.IsDir() || IsSymlink(fi) {
		return true
	}
	return false
}
