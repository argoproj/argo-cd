package main

import (
	"fmt"
	"golang.org/x/mod/semver"
	"os"
	"os/exec"
	"regexp"
	"strconv"
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
	proposedMajor := semver.Major(proposedTag)
	proposedMinor := semver.MajorMinor(proposedTag)

	proposedPatch, proposedRC, err := extractPatchAndRC(proposedTag)
	if err != nil {
		return "", err
	}

	// If the current tag is a .0 patch release or a 1 release candidate, adjust to the previous minor release series.
	if (proposedPatch == "0" && proposedRC == "0") || proposedRC == "1" {
		proposedMinorInt, err := strconv.Atoi(strings.TrimPrefix(proposedMinor, proposedMajor+"."))
		if err != nil {
			return "", fmt.Errorf("invalid minor version: %v", err)
		}
		if proposedMinorInt > 0 {
			proposedMinor = fmt.Sprintf("%s.%d", proposedMajor, proposedMinorInt-1)
		}
	}

	for _, tag := range tags {
		if tag == proposedTag {
			continue
		}
		tagMajor := semver.Major(tag)
		tagMinor := semver.MajorMinor(tag)
		tagPatch, tagRC, err := extractPatchAndRC(tag)
		if err != nil {
			continue
		}

		// Only bother considering tags with the same major and minor version.
		if tagMajor == proposedMajor && tagMinor == proposedMinor {
			// If it's a non-RC release...
			if proposedRC == "0" {
				// Only consider non-RC tags.
				if tagRC == "0" {
					if semver.Compare(tag, previousTag) > 0 {
						previousTag = tag
					}
				}
			} else {
				if tagRC != "0" && tagPatch == proposedPatch {
					if semver.Compare(tag, previousTag) > 0 {
						previousTag = tag
					}
				} else if tagRC == "0" {
					if semver.Compare(tag, previousTag) > 0 {
						previousTag = tag
					}
				}
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
