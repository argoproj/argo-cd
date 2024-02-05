package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

func main() {
	err := release()
	if err != nil {
		fmt.Printf("Failed to release: %s\n", err.Error())
	}
}

func getCurrentCommitSha() (string, error) {
	// git rev-parse --short HEAD
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	result, err := cmd.Output()
	if err != nil {
		return "", err
	}

	rs := strings.Split(string(result), "\n")
	return strings.Split(rs[0], " ")[0], nil
}

func getArgoCDVersion() (string, error) {
	// git rev-parse --abbrev-ref HEAD
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	result, err := cmd.Output()
	if err != nil {
		return "", err
	}

	pattern := `release-(\d+\.\d+)`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(string(result))
	if len(matches) >= 2 {
		return matches[1], nil
	}

	return "", fmt.Errorf("failed to get argocd version")
}

// function that returns version of the latest release by patern
// {argocd-version}-{timestamp}-{commit-sha}
func getLatestVersion() (string, error) {
	commitSha, err := getCurrentCommitSha()
	if err != nil {
		return "", err
	}

	argocdVersion, err := getArgoCDVersion()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s-%s-%s", argocdVersion, time.Now().Format("2006.1.2"), commitSha), nil
}

func updateVersion(version string) error {
	file, err := os.OpenFile("VERSION", os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write the new content to the file
	_, err = file.WriteString(version)
	if err != nil {
		return err
	}

	return nil
}

func readChangelog() (string, error) {
	data, err := os.ReadFile("changelog/CHANGELOG.md")
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func moveChangelog() error {
	version, err := getLatestVersion()
	if err != nil {
		return err
	}

	// mv changelog/CHANGELOG.md changelog/CHANGELOG-<version>.md
	cmd := exec.Command("mv", "changelog/CHANGELOG.md", fmt.Sprintf("changelog/CHANGELOG-%s.md", version))
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Print(string(output))
		return err
	}

	return nil
}

func release() error {
	version, err := getLatestVersion()
	if err != nil {
		return err
	}

	fmt.Printf("Releasing version: %s\n", version)
	err = updateVersion(version)
	if err != nil {
		return err
	}

	changelog, err := readChangelog()
	if err != nil {
		return err
	}

	fmt.Printf("Changelog: %s\n", changelog)
	release := fmt.Sprintf("release-v%s", version)
	fmt.Printf("Release: %s\n", release)
	err = moveChangelog()
	if err != nil {
		return err
	}

	err = commitChanges(version)
	if err != nil {
		return err
	}

	// git tag -a v2.9.3-2021.07.07-3a4b7f4 -m "Codefresh version for synced 2.9.3"
	_ = exec.Command("git", "tag", "-d", release).Run()
	cmd := exec.Command("git", "tag", "-a", release, "-m", changelog)
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Print(string(output))
		return fmt.Errorf("failed to tag: %w", err)
	}

	// git push remote-name --delete tag-name
	_ = exec.Command("git", "push", "origin", "--delete", release).Run()
	// git push origin tags/version
	cmd = exec.Command("git", "push", "origin", "tags/"+release)
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Print(string(output))
		return fmt.Errorf("failed to push tag: %w", err)
	}

	return exec.Command("git", "push", "origin", "--delete", release).Run()
}

func commitChanges(version string) error {
	// git add VERSION
	cmd := exec.Command("git", "add", "VERSION")
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Print(string(output))
		return fmt.Errorf("failed to add version: %w", err)
	}

	cmd = exec.Command("git", "add", fmt.Sprintf("changelog/CHANGELOG-%s.md", version))
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Print(string(output))
		return fmt.Errorf("failed to add changelog: %w", err)
	}

	cmd = exec.Command("git", "commit", "-a", "-m", fmt.Sprintf("chore: update version to %s", version))
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Print(string(output))
		return fmt.Errorf("failed to commit changes: %w", err)
	}

	cmd = exec.Command("git", "push")
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Print(string(output))
		return fmt.Errorf("failed to push changes: %w", err)
	}

	return nil
}
