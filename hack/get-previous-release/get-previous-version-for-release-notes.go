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

func removeInvalidTags(tags []string) []string {
	var validTags []string
	for _, tag := range tags {
		if _, _, err := extractPatchAndRC(tag); err == nil {
			validTags = append(validTags, tag)
		}
	}
	return validTags
}

func removeNewerTags(proposedTag string, tags []string) []string {
	var validTags []string
	for _, tag := range tags {
		if semver.Compare(tag, proposedTag) <= 0 {
			validTags = append(validTags, tag)
		}
	}
	return validTags
}

func removeTagsFromSameMinorSeries(proposedTag string, tags []string) []string {
	var validTags []string
	proposedMinor := semver.MajorMinor(proposedTag)
	for _, tag := range tags {
		if semver.MajorMinor(tag) != proposedMinor {
			validTags = append(validTags, tag)
		}
	}
	return validTags
}

func getMostRecentTag(tags []string) string {
	var mostRecentTag string
	for _, tag := range tags {
		if mostRecentTag == "" || semver.Compare(tag, mostRecentTag) > 0 {
			mostRecentTag = tag
		}
	}
	return mostRecentTag
}

func findPreviousTag(proposedTag string, tags []string) (string, error) {
	proposedPatch, proposedRC, err := extractPatchAndRC(proposedTag)
	if err != nil {
		return "", err
	}

	tags = removeInvalidTags(tags)
	tags = removeNewerTags(proposedTag, tags)

	if proposedRC == "0" && proposedPatch == "0" {
		// If we're cutting the first patch of a new minor release series, don't consider tags in the same minor release
		// series.
		tags = removeTagsFromSameMinorSeries(proposedTag, tags)
	}

	previousTag := getMostRecentTag(tags)
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
