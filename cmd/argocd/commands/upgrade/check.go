package upgrade

import (
	"errors"
	"fmt"
)

const (
	checkPass int = 0
	checkFail int = 1
	checkWarn int = 2
	checkInfo int = 3
)

type Check interface {
	performChecks(u *Upgrade) (checklist []CheckResult, err error)
}

type Rule struct {
	title   string
	actions []string
	result  int
}

type CheckResult struct {
	title       string
	description string
	rules       []Rule
}

var getCheck = func(checkType string) (Check, error) {
	switch checkType {
	case "v2v3":
		return new(V2V3Check), nil
	default:
		return nil, errors.New("no checks found for this upgrade")
	}
}

func printChecks(checkList []CheckResult) error {
	if len(checkList) == 0 {
		return errors.New("no checks found for this upgrade")
	}
	for i, checkResult := range checkList {
		printHeader(i+1, len(checkList), checkResult)
		printRules(checkResult.rules)
	}

	printChecklistSummary(checkList)

	return nil
}

func printHeader(checkNumber int, totalChecks int, checkResult CheckResult) {
	fmt.Print("\n")
	printBanner("=", 80)
	fmt.Printf("Check : %s\n\n", checkResult.title)
	fmt.Printf("%s\n", checkResult.description)
	printBanner("=", 80)
	fmt.Printf("Check %d of %d\n", checkNumber, totalChecks)
	fmt.Printf("Total Rules: %d\n", len(checkResult.rules))
}

func printRules(rules []Rule) {
	if len(rules) == 0 {
		fmt.Printf("No rules found for this check.\n")
		return
	}
	printRulesSummary(rules)
	for _, rule := range rules {
		printBanner("-", 80)
		fmt.Printf("Rule  : %s\n", rule.title)
		fmt.Printf("Result: %s\n", getResult(rule.result))
		for _, action := range rule.actions {
			fmt.Printf("Action: %s\n", action)
		}
	}
}

func getResult(resultNumber int) (result string) {
	switch resultNumber {
	case checkPass:
		result = "PASS"
	case checkFail:
		result = "FAIL"
	case checkWarn:
		result = "WARN"
	case checkInfo:
		result = "INFO"
	default:
		result = "UNKNOWN"
	}
	return result
}

func printRulesSummary(rules []Rule) {
	pass := 0
	fail := 0
	warn := 0
	info := 0
	for _, rule := range rules {
		switch rule.result {
		case checkPass:
			pass++
		case checkFail:
			fail++
		case checkWarn:
			warn++
		case checkInfo:
			info++
		}
	}
	fmt.Print("Results: \n")
	fmt.Printf("  Pass: %d\n", pass)
	fmt.Printf("  Fail: %d\n", fail)
	fmt.Printf("  Warn: %d\n", warn)
	fmt.Printf("  Info: %d\n", info)
}

func printChecklistSummary(checkList []CheckResult) {
	totalRules := 0
	totalPass := 0
	totalFail := 0
	totalWarn := 0
	totalInfo := 0

	printBanner("=", 80)
	fmt.Print("Upgrade Summary\n")
	printBanner("=", 80)
	fmt.Printf("Total Checks: %d\n", len(checkList))
	for _, checkResult := range checkList {
		totalRules += len(checkResult.rules)
		for _, rule := range checkResult.rules {
			switch rule.result {
			case checkPass:
				totalPass++
			case checkFail:
				totalFail++
			case checkWarn:
				totalWarn++
			case checkInfo:
				totalInfo++
			}
		}
	}
	fmt.Printf("Total Rules: %d\n", totalRules)
	fmt.Print("Total Results:\n")
	fmt.Printf("  Pass: %d\n", totalPass)
	fmt.Printf("  Fail: %d\n", totalFail)
	fmt.Printf("  Warn: %d\n", totalWarn)
	fmt.Printf("  Info: %d\n", totalInfo)
}

func printBanner(character string, width int) {
	for i := 0; i < width; i++ {
		fmt.Print(character)
	}
	fmt.Print("\n")
}
