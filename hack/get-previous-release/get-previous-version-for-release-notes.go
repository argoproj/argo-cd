package main

import (
	"fmt"
	"golang.org/x/mod/semver"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

/**
This script is used to determine the previous version of a release based on the current version. It is used to help
generate release notes for a new release.
*/

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run get-previous-version-for-release-notes.go <version being released>")
		return
	}

	proposedTag := os.Args[1]

	tags, err := getGitTags()
	if err != nil {
		fmt.Printf("Error getting git tags: %v\n", err)
		return
	}

	previousTag, err := findPreviousTag(proposedTag, tags)
	if err != nil {
		fmt.Printf("Error finding previous tag: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%s\n", previousTag)
}

func extractPatchAndRC(tag string) (string, string, error) {
	re := regexp.MustCompile(`^v\d+\.\d+\.(\d+)(?:-rc(\d+))?$`)
	matches := re.FindStringSubmatch(tag)
	if len(matches) < 2 {
		return "", "", fmt.Errorf("invalid tag format: %s", tag)
	}
	patch := matches[1]
	rc := "0"
	if len(matches) == 3 && matches[2] != "" {
		rc = matches[2]
	}
	return patch, rc, nil
}

func findPreviousTag(proposedTag string, tags []string) (string, error) {
	var previousTag string
	proposedMinor := semver.MajorMinor(proposedTag)

	proposedPatch, proposedRC, err := extractPatchAndRC(proposedTag)
	if err != nil {
		return "", err
	}

	for _, tag := range tags {
		// If this tag is newer than the proposed tag, skip it.
		if semver.Compare(tag, proposedTag) > 0 {
			continue
		}

		// If this tag is older than a tag we've already decided is a candidate, skip it.
		if semver.Compare(tag, previousTag) <= 0 {
			continue
		}
		tagPatch, tagRC, err := extractPatchAndRC(tag)
		if err != nil {
			continue
		}

		// If it's a non-RC release...
		if proposedRC == "0" {
			if proposedPatch == "0" {
				// ...and we're cutting the first patch of a new minor release series, don't consider tags in the same
				// minor release series.
				if semver.MajorMinor(tag) != proposedMinor {
					previousTag = tag
				}
			} else {

				previousTag = tag
			}
		} else {
			if tagRC != "0" && tagPatch == proposedPatch {
				previousTag = tag
			} else if tagRC == "0" {
				previousTag = tag
			}
		}
	}
	if previousTag == "" {
		return "", fmt.Errorf("no matching tag found for tags: " + strings.Join(tags, ", "))
	}
	return previousTag, nil
}

func getGitTags() ([]string, error) {
	cmd := exec.Command("git", "tag", "--sort=-v:refname")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error executing git command: %v", err)
	}

	tags := strings.Split(string(output), "\n")
	var semverTags []string
	for _, tag := range tags {
		if semver.IsValid(tag) {
			semverTags = append(semverTags, tag)
		}
	}

	return semverTags, nil
}
