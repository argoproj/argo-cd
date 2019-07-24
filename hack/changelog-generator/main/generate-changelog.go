package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type Issue struct {
	Number int
	Title  string
}

func main() {

	labels := make(map[int]string, 0)
	for _, label := range []string{"bug", "enhancement"} {
		for _, issue := range issuesByLabel(label) {
			labels[issue.Number] = label
		}
	}

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
	branchIssueNums := make(map[string][]int)
	{
		for _, branch := range branches {
			for _, issue := range getIssues(branch) {
				_, ok := issues[issue.Number]
				if !ok {
					issues[issue.Number] = issue
					branchIssueNums[branch] = append(branchIssueNums[branch], issue.Number)
				}
			}
		}
	}
	for i := len(branches)/2 - 1; i >= 0; i-- {
		opp := len(branches) - 1 - i
		branches[i], branches[opp] = branches[opp], branches[i]
	}
	fmt.Println(labels)
	{
		fmt.Println("# Changelog")
		for _, branch := range branches {
			fmt.Printf("## v%s\n", strings.TrimPrefix(branch, "origin/release-"))
			fmt.Printf("%d issue(s)\n\n", len(branchIssueNums[branch]))
			for _, num := range branchIssueNums[branch] {
				link := fmt.Sprintf("[#%d](https://github.com/argoproj/argo-cd/issues/%d)", num, num)
				subject := strings.Replace(issues[num].Title, fmt.Sprintf("#%d", num), link, -1)
				fmt.Printf("* %s %v\n", subject, labels[num])
			}
			fmt.Println()
		}
	}
}

func issuesByLabel(label string) []Issue {
	client := &http.Client{}
	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/argoproj/argo-cd/issues?filter=all&state=closed&per_page=9999&label=%s", label), nil)
	if err != nil {
		panic(err)
	}
	//req.Header.Set("Authorization", "Basic YWxleGVjOmQ3YmIzODBjYTVlZWVkNGYxMDhiYWRhYjU2OTgyNmIzZmY0NzJhOWM=")
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 {
		panic(resp.Status)
	}
	issues := &([]Issue{})
	err = json.NewDecoder(resp.Body).Decode(issues)
	if err != nil {
		panic(err)
	}

	fmt.Println(issues)
	return *issues
}

func issueNum(subject string) int {
	rx := regexp.MustCompile("#[0-9][0-9]+")
	match := rx.Find([]byte(subject))
	issueId, _ := strconv.ParseInt(strings.TrimPrefix(string(match), "#"), 10, 0)
	return int(issueId)
}

func getIssues(branch string) []Issue {
	text, _ := exec.Command("git", "log", "--format=%s", branch).Output()
	var issues []Issue
	for _, subject := range strings.Split(string(text), "\n") {
		num := issueNum(subject)
		if num > 0 {
			issues = append(issues, Issue{num, subject})
		}
	}
	return issues
}
