package files

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	log "github.com/sirupsen/logrus"
)

// globMetaChars are the characters that make filepath.Match treat a pattern as
// a glob rather than a literal path.
const globMetaChars = `*?[`

// maxSymlinkExpansionRounds caps how many times include paths are expanded with
// symlink targets. Every round can select new directories holding symlinks of
// their own, but symlink chains are shallow in practice.
const maxSymlinkExpansionRounds = 5

type tgz struct {
	srcPath      string
	inclusions   []string
	includePaths []string
	exclusions   []string
	tarWriter    *tar.Writer
	filesWritten int
}

// TarOptions configures which of the files under srcPath end up in the archive.
// An empty TarOptions archives everything.
type TarOptions struct {
	// Inclusions restricts the archive to files whose base name matches one of
	// the given filepath.Match patterns. Directories are always descended into.
	Inclusions []string
	// IncludePaths restricts the archive to the given paths, relative to
	// srcPath. A path selects itself, everything below it when it is a
	// directory, and everything matching it when it holds glob metacharacters.
	// Directories that cannot hold a selected path are not walked at all, and
	// the targets of selected symlinks are selected as well.
	IncludePaths []string
	// Exclusions drops files whose path relative to srcPath matches one of the
	// given filepath.Match patterns.
	Exclusions []string
}

// Tgz will iterate over all files found in srcPath compressing them with gzip
// and archiving with Tar. Will invoke every given writer while generating the tgz.
// This is useful to generate checksums. Will exclude files matching the exclusions
// list blob if exclusions is not nil. Will include only the files matching the
// inclusions list if inclusions is not nil.
func Tgz(srcPath string, inclusions []string, exclusions []string, writers ...io.Writer) (int, error) {
	return TgzWithOptions(srcPath, TarOptions{Inclusions: inclusions, Exclusions: exclusions}, writers...)
}

// TgzWithOptions behaves like Tgz, filtering the archived files as described by opts.
func TgzWithOptions(srcPath string, opts TarOptions, writers ...io.Writer) (int, error) {
	if _, err := os.Stat(srcPath); err != nil {
		return 0, fmt.Errorf("error inspecting srcPath %q: %w", srcPath, err)
	}

	gzw := gzip.NewWriter(io.MultiWriter(writers...))
	defer gzw.Close()

	return writeFile(srcPath, opts, gzw)
}

// Tar will iterate over all files found in srcPath archiving with Tar. Will invoke every given writer while generating the tar.
// This is useful to generate checksums. Will exclude files matching the exclusions
// list blob if exclusions is not nil. Will include only the files matching the
// inclusions list if inclusions is not nil.
func Tar(srcPath string, inclusions []string, exclusions []string, writers ...io.Writer) (int, error) {
	return TarWithOptions(srcPath, TarOptions{Inclusions: inclusions, Exclusions: exclusions}, writers...)
}

// TarWithOptions behaves like Tar, filtering the archived files as described by opts.
func TarWithOptions(srcPath string, opts TarOptions, writers ...io.Writer) (int, error) {
	if _, err := os.Stat(srcPath); err != nil {
		return 0, fmt.Errorf("error inspecting srcPath %q: %w", srcPath, err)
	}

	return writeFile(srcPath, opts, io.MultiWriter(writers...))
}

func writeFile(srcPath string, opts TarOptions, writer io.Writer) (int, error) {
	tw := tar.NewWriter(writer)
	defer tw.Close()

	t := &tgz{
		srcPath:      srcPath,
		inclusions:   opts.Inclusions,
		includePaths: expandIncludePaths(srcPath, opts.IncludePaths),
		exclusions:   opts.Exclusions,
		tarWriter:    tw,
	}
	err := filepath.Walk(srcPath, t.tgzFile)
	if err != nil {
		return 0, err
	}

	return t.filesWritten, nil
}

// Untgz will loop over the tar reader creating the file structure at dstPath.
// Callers must make sure dstPath is:
//   - a full path
//   - points to an empty directory or
//   - points to a non-existing directory
func Untgz(dstPath string, r io.Reader, maxSize int64, preserveFileMode bool) error {
	if !filepath.IsAbs(dstPath) {
		return fmt.Errorf("dstPath points to a relative path: %s", dstPath)
	}

	gzr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}
	defer gzr.Close()
	return untar(dstPath, io.LimitReader(gzr, maxSize), preserveFileMode)
}

// Untar will loop over the tar reader creating the file structure at dstPath.
// Callers must make sure dstPath is:
//   - a full path
//   - points to an empty directory or
//   - points to a non-existing directory
func Untar(dstPath string, r io.Reader, maxSize int64, preserveFileMode bool) error {
	if !filepath.IsAbs(dstPath) {
		return fmt.Errorf("dstPath points to a relative path: %s", dstPath)
	}

	return untar(dstPath, io.LimitReader(r, maxSize), preserveFileMode)
}

// untar will loop over the tar reader creating the file structure at dstPath.
// Callers must make sure dstPath is:
//   - a full path
//   - points to an empty directory or
//   - points to a non existing directory
func untar(dstPath string, r io.Reader, preserveFileMode bool) error {
	tr := tar.NewReader(r)

	for {
		header, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("error while iterating on tar reader: %w", err)
		}
		if header == nil || header.Name == "." || header.Name == "./" {
			continue
		}

		target := filepath.Join(dstPath, header.Name)
		// Sanity check to protect against zip-slip
		if !Inbound(target, dstPath) {
			return fmt.Errorf("illegal filepath in archive: %s", target)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			var mode os.FileMode = 0o755
			if preserveFileMode {
				mode = os.FileMode(header.Mode)
			}
			err := os.MkdirAll(target, mode)
			if err != nil {
				return fmt.Errorf("error creating nested folders: %w", err)
			}
		case tar.TypeSymlink:
			// Sanity check to protect against symlink exploit
			linkTarget := filepath.Join(filepath.Dir(target), header.Linkname)
			realLinkTarget, err := filepath.EvalSymlinks(linkTarget)
			if os.IsNotExist(err) {
				realLinkTarget = linkTarget
			} else if err != nil {
				return fmt.Errorf("error checking symlink realpath: %w", err)
			}
			if !Inbound(realLinkTarget, dstPath) {
				return fmt.Errorf("illegal filepath in symlink: %s", linkTarget)
			}

			// Relativizing all symlink targets because path.CheckOutOfBoundsSymlinks disallows any absolute symlinks
			// and it makes more sense semantically to view symlinks in archives as relative.
			// Inbound ensures that we never allow symlinks that break out of the target directory.
			realLinkTarget, err = filepath.Rel(filepath.Dir(target), realLinkTarget)
			if err != nil {
				return fmt.Errorf("error relativizing link target: %w", err)
			}

			err = os.Symlink(realLinkTarget, target)
			if err != nil {
				return fmt.Errorf("error creating symlink: %w", err)
			}
		case tar.TypeReg:
			var mode os.FileMode = 0o644
			if preserveFileMode {
				mode = os.FileMode(header.Mode)
			}

			err := os.MkdirAll(filepath.Dir(target), 0o755)
			if err != nil {
				return fmt.Errorf("error creating nested folders: %w", err)
			}

			f, err := os.OpenFile(target, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode)
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

// pathSelected reports whether relativePath, a path relative to the archive
// root, is selected by one of the given include paths. An include path selects
// itself, anything below it, and anything its glob matches. A glob may name a
// directory, so it is matched against the parent directories as well.
func pathSelected(relativePath string, includePaths []string) bool {
	sep := string(filepath.Separator)
	for _, includePath := range includePaths {
		if includePath == "" || includePath == "." {
			return true
		}
		if relativePath == includePath || strings.HasPrefix(relativePath, includePath+sep) {
			return true
		}
		if !strings.ContainsAny(includePath, globMetaChars) {
			continue
		}
		for p := relativePath; p != "." && p != sep; p = filepath.Dir(p) {
			if matched, err := filepath.Match(includePath, p); err == nil && matched {
				return true
			}
		}
	}
	return false
}

// dirMayHoldSelected reports whether the directory at dirRel can hold anything
// selected by includePaths, which lets the walk skip unrelated subtrees instead
// of scanning them. Globs are pruned by the literal prefix preceding their first
// metacharacter, as that is the most that can be compared without matching.
func dirMayHoldSelected(dirRel string, includePaths []string) bool {
	if dirRel == "." {
		return true
	}
	sep := string(filepath.Separator)
	for _, includePath := range includePaths {
		if includePath == "" || includePath == "." {
			return true
		}
		literal := includePath
		if i := strings.IndexAny(includePath, globMetaChars); i >= 0 {
			literal = strings.TrimSuffix(includePath[:i], sep)
			if literal == "" {
				// The pattern may match at any depth.
				return true
			}
		}
		// The directory either holds the include path, is the include path, or
		// is on the way down to it.
		if dirRel == literal ||
			strings.HasPrefix(dirRel, literal+sep) ||
			strings.HasPrefix(literal, dirRel+sep) {
			return true
		}
	}
	return false
}

// expandIncludePaths returns includePaths plus the targets of any symlink they
// select. A symlink archived without its target usually breaks the consumer of
// the archive, and a target living under the archive root is implicitly part of
// what the caller selected.
func expandIncludePaths(srcPath string, includePaths []string) []string {
	if len(includePaths) == 0 {
		return includePaths
	}

	expanded := slices.Clone(includePaths)
	selected := make(map[string]bool, len(expanded))
	for _, includePath := range expanded {
		selected[includePath] = true
	}

	for range maxSymlinkExpansionRounds {
		added := false
		for _, target := range selectedSymlinkTargets(srcPath, expanded) {
			if selected[target] {
				continue
			}
			selected[target] = true
			expanded = append(expanded, target)
			added = true
		}
		if !added {
			break
		}
	}
	return expanded
}

// selectedSymlinkTargets returns the targets, relative to srcPath, of the
// symlinks under srcPath that includePaths selects. Targets pointing outside
// srcPath are skipped as they can never be part of the archive.
func selectedSymlinkTargets(srcPath string, includePaths []string) []string {
	var targets []string
	err := filepath.Walk(srcPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error walking in %q: %w", srcPath, err)
		}
		relativePath, err := RelativePath(path, srcPath)
		if err != nil {
			return fmt.Errorf("relative path error: %w", err)
		}
		if relativePath == "." {
			return nil
		}
		if fi.IsDir() {
			if !dirMayHoldSelected(relativePath, includePaths) {
				return filepath.SkipDir
			}
			return nil
		}
		if !IsSymlink(fi) || !pathSelected(relativePath, includePaths) {
			return nil
		}

		link, err := os.Readlink(path)
		if err != nil {
			log.Warnf("error reading link %q: %v", path, err)
			return nil
		}
		target := link
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), link)
		}
		targetRel, err := RelativePath(target, srcPath)
		if err != nil || targetRel == ".." || strings.HasPrefix(targetRel, ".."+string(filepath.Separator)) {
			log.Debugf("symlink %q targets %q outside of %q, not adding it to the archive", relativePath, link, srcPath)
			return nil
		}
		targets = append(targets, targetRel)
		return nil
	})
	if err != nil {
		// Losing a target only means it is not added to the archive, which the
		// main walk reports as a missing file if it turns out to be needed.
		log.Warnf("error looking for symlink targets in %q: %v", srcPath, err)
	}
	return targets
}

// tgzFile is used as a filepath.WalkFunc implementing the logic to write
// the given file in the tgz.tarWriter applying the exclusion pattern defined
// in tgz.exclusions, or the inclusion patterns defined in tgz.inclusions and
// tgz.includePaths.
// Only regular files will be added in the tarball.
func (t *tgz) tgzFile(path string, fi os.FileInfo, err error) error {
	if err != nil {
		return fmt.Errorf("error walking in %q: %w", t.srcPath, err)
	}

	base := filepath.Base(path)

	relativePath, err := RelativePath(path, t.srcPath)
	if err != nil {
		return fmt.Errorf("relative path error: %w", err)
	}

	if len(t.includePaths) > 0 && relativePath != "." {
		if fi.IsDir() {
			if !dirMayHoldSelected(relativePath, t.includePaths) {
				return filepath.SkipDir
			}
		} else if !pathSelected(relativePath, t.includePaths) {
			return nil
		}
	}
	if t.inclusions != nil && base != "." && !fi.IsDir() {
		included := false
		for _, inclusionPattern := range t.inclusions {
			found, err := filepath.Match(inclusionPattern, base)
			if err != nil {
				return fmt.Errorf("error verifying inclusion pattern %q: %w", inclusionPattern, err)
			}
			if found {
				included = true
				break
			}
		}
		if !included {
			return nil
		}
	}
	if t.exclusions != nil {
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
	}

	if !supportedFileMode(fi) {
		return nil
	}

	link := ""
	if IsSymlink(fi) {
		link, err = os.Readlink(path)
		if err != nil {
			return fmt.Errorf("error getting link target: %w", err)
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
		defer func() {
			err := f.Close()
			if err != nil {
				log.Errorf("error closing file %q: %v", fi.Name(), err)
			}
		}()

		if _, err := io.Copy(t.tarWriter, f); err != nil {
			return fmt.Errorf("error copying tgz file to writers: %w", err)
		}
		t.filesWritten++
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
