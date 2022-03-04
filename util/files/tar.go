package files

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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

// Untgz will loop over the tar reader creating the file structure at dstPath
func Untgz(dstPath string, r io.Reader) error {

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
		if header == nil {
			continue
		}

		target := filepath.Join(dstPath, header.Name)

		// Sanity check to protect against zip-slip
		if !strings.HasPrefix(target, filepath.Clean(dstPath)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal filepath in archive: %s", target)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			err := createNestedFolders(target)
			if err != nil {
				return fmt.Errorf("error creating nested folders: %w", err)
			}
		case tar.TypeReg:
			err := createNestedFolders(filepath.Dir(target))
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

// createAllFolders will create all remaining folders from the given path.
// No-op if path already exists.
func createNestedFolders(path string) error {
	_, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := os.MkdirAll(path, 0755); err != nil {
				return fmt.Errorf("error creating dir %q: %w", path, err)
			}
		} else {
			return fmt.Errorf("error inspecting nested folder %q: %w", path, err)
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

	relativePath := RelativePath(path, t.srcPath)

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

	if !fi.Mode().IsRegular() {
		return nil
	}

	header, err := tar.FileInfoHeader(fi, fi.Name())
	if err != nil {
		return fmt.Errorf("error creating a tar file header: %w", err)
	}

	// update the name to correctly reflect the desired destination when untaring
	header.Name = relativePath

	if err := t.tarWriter.WriteHeader(header); err != nil {
		return fmt.Errorf("error writing header: %w", err)
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("error opening file %q: %w", fi.Name(), err)
	}
	defer f.Close()

	if _, err := io.Copy(t.tarWriter, f); err != nil {
		return fmt.Errorf("error copying tgz file to writters: %w", err)
	}

	return nil
}
