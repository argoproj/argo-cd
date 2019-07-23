package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type Issue struct {
	id      int
	subject string
}

func main() {
	fmt.Println("# Changelog")
	var branches []string
	{
		text, _ := exec.Command("git", "branch", "--list", "--remotes", "--sort=committerdate").Output()
		for _, branch := range strings.Split(string(text), "\n") {
			if !strings.Contains(branch, "origin/release-") {
				continue
			}
			branch = strings.TrimSpace(branch)
			branches = append(branches, branch)
		}
	}
	issues := make(map[int]Issue)
	branchIssueIds := make(map[string][]int)
	{
		for _, branch := range branches {
			for _, issue := range getIssues(branch) {
				_, ok := issues[issue.id]
				if !ok {
					issues[issue.id] = issue
					branchIssueIds[branch] = append(branchIssueIds[branch], issue.id)
				}
			}
		}
	}
	for i := len(branches)/2 - 1; i >= 0; i-- {
		opp := len(branches) - 1 - i
		branches[i], branches[opp] = branches[opp], branches[i]
	}
	{
		for _, branch := range branches {
			fmt.Printf("## v%s\n\n", strings.TrimPrefix(branch, "origin/release-"))
			fmt.Printf("%d issue(s)", len(branchIssueIds[branch]))
			for _, issueId := range branchIssueIds[branch] {
				link := fmt.Sprintf("[#%d](https://github.com/argoproj/argo-cd/issues/%d)", issueId, issueId)
				fmt.Printf("* %s\n", strings.Replace(issues[issueId].subject, fmt.Sprintf("#%d", issueId), link, -1))
			}
			fmt.Println()
		}
	}

}

func issueId(subject string) int {
	rx := regexp.MustCompile("#[0-9][0-9]+")
	split := rx.FindAll([]byte(subject), 3)
	if len(split) == 0 {
		return 0
	}
	match := split[len(split)-1]
	issueId, _ := strconv.ParseInt(strings.TrimPrefix(string(match), "#"), 10, 0)
	return int(issueId)
}

func getIssues(branch string) []Issue {
	text, _ := exec.Command("git", "log", "--format=%s", branch).Output()
	var issues []Issue
	for _, subject := range strings.Split(string(text), "\n") {
		issueId := issueId(subject)
		if issueId > 0 {
			issues = append(issues, Issue{issueId, subject})
		}
	}
	return issues
}
