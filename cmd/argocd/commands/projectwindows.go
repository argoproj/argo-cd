package commands

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	projectpkg "github.com/argoproj/argo-cd/pkg/apiclient/project"
	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util"
)

// NewProjectWindowsCommand returns a new instance of the `argocd proj windows` command
func NewProjectWindowsCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	roleCommand := &cobra.Command{
		Use:   "windows",
		Short: "Manage a project's sync windows",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}
	roleCommand.AddCommand(NewProjectWindowsDisableManualSyncCommand(clientOpts))
	roleCommand.AddCommand(NewProjectWindowsEnableManualSyncCommand(clientOpts))
	roleCommand.AddCommand(NewProjectWindowsAddWindowCommand(clientOpts))
	roleCommand.AddCommand(NewProjectWindowsDeleteCommand(clientOpts))
	roleCommand.AddCommand(NewProjectWindowsListCommand(clientOpts))
	roleCommand.AddCommand(NewProjectWindowsUpdateCommand(clientOpts))
	roleCommand.AddCommand(NewProjectWindowsRulesCommand(clientOpts))
	return roleCommand
}

// NewProjectWindowsRuleCommand returns a new instance of the `argocd proj windows rules` command
func NewProjectWindowsRulesCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	roleCommand := &cobra.Command{
		Use:   "rules",
		Short: "Manage a project's sync windows rules",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}
	roleCommand.AddCommand(NewProjectWindowsRulesAddCommand(clientOpts))
	roleCommand.AddCommand(NewProjectWindowsRulesDeleteCommand(clientOpts))
	roleCommand.AddCommand(NewProjectWindowsRulesListCommand(clientOpts))
	roleCommand.AddCommand(NewProjectWindowsRulesAddConditionCommand(clientOpts))
	roleCommand.AddCommand(NewProjectWindowsRulesDeleteConditionCommand(clientOpts))
	return roleCommand
}

// NewProjectSyncWindowsDisableManualSyncCommand returns a new instance of an `argocd proj windows disable-manual-sync` command
func NewProjectWindowsDisableManualSyncCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "disable-manual-sync PROJECT WINDOW_ID",
		Short: "Disable manual sync for a sync window",
		Long:  "Disable manual sync for a sync window. Requires window ID which can be found by running \"argocd proj windows list PROJECT\"",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			projName := args[0]
			windowID, err := strconv.Atoi(args[1])
			errors.CheckError(err)

			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			proj, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			for i, window := range proj.Spec.SyncWindows {
				if windowID == i {
					window.ManualSync = false
				}
			}

			_, err = projIf.Update(context.Background(), &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	return command
}

// NewProjectWindowsEnableManualSyncCommand returns a new instance of an `argocd proj windows enable-manual-sync` command
func NewProjectWindowsEnableManualSyncCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "enable-manual-sync PROJECT WINDOW_ID",
		Short: "Enable manual sync for a sync window",
		Long:  "Enable manual sync for a sync window. To get WINDOW_ID run \"argocd proj windows list PROJECT\"",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			projName := args[0]
			windowID, err := strconv.Atoi(args[1])
			errors.CheckError(err)

			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			proj, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			for i, window := range proj.Spec.SyncWindows {
				if windowID == i {
					window.ManualSync = true
				}
			}

			_, err = projIf.Update(context.Background(), &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	return command
}

// NewProjectWindowsAddWindowCommand returns a new instance of an `argocd proj windows add` command
func NewProjectWindowsAddWindowCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		window     argoappv1.SyncWindow
		conditions []string
	)
	var command = &cobra.Command{
		Use:   "add PROJECT",
		Short: "Add a sync window to a project",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			proj, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			rule, err := generateRule(conditions)
			errors.CheckError(err)

			rules := argoappv1.WindowRules{*rule}

			window.Rules = rules

			err = proj.Spec.AddWindow(&window)
			errors.CheckError(err)

			_, err = projIf.Update(context.Background(), &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	command.Flags().StringVarP(&window.Kind, "kind", "k", "", "Sync window kind, either allow or deny")
	command.Flags().StringVar(&window.Schedule, "schedule", "", "Sync window schedule in cron format. (e.g. --schedule \"0 22 * * *\")")
	command.Flags().StringVar(&window.Duration, "duration", "", "Sync window duration. (e.g. --duration 1h)")
	command.Flags().StringArrayVar(&conditions, "condition", []string{}, "Condition for matching the rule, support for multiple conditions per rule. (e.g. --condition \"application in (web-*,db1)\")")
	command.Flags().BoolVar(&window.ManualSync, "manual-sync", false, "Allow manual syncs for both deny and allow windows")
	return command
}

// NewProjectWindowsDeleteCommand returns a new instance of an `argocd proj windows delete` command
func NewProjectWindowsDeleteCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "delete PROJECT WINDOW_ID",
		Short: "delete a sync window.",
		Long:  "delete a sync window. To get WINDOW_ID run \"argocd proj windows list PROJECT\"",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			projName := args[0]
			windowID, err := strconv.Atoi(args[1])
			errors.CheckError(err)

			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			proj, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			err = proj.Spec.DeleteWindow(windowID)
			errors.CheckError(err)

			_, err = projIf.Update(context.Background(), &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	return command
}

// NewProjectWindowsUpdateCommand returns a new instance of an `argocd proj windows update` command
func NewProjectWindowsUpdateCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		kind     string
		schedule string
		duration string
	)
	var command = &cobra.Command{
		Use:   "update PROJECT WINDOW_ID",
		Short: "Update a project sync window",
		Long:  "Update a project sync window. To get WINDOW_ID run \"argocd proj windows list PROJECT\"",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			projName := args[0]
			windowID, err := strconv.Atoi(args[1])
			errors.CheckError(err)

			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			proj, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			for i, window := range proj.Spec.SyncWindows {
				if windowID == i {
					err := window.Update(kind, schedule, duration)
					if err != nil {
						errors.CheckError(err)
					}
				}
			}

			_, err = projIf.Update(context.Background(), &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	command.Flags().StringVar(&kind, "kind", "", "Sync window kind, either allow or deny")
	command.Flags().StringVar(&schedule, "schedule", "", "Sync window schedule in cron format. (e.g. --schedule \"0 22 * * *\")")
	command.Flags().StringVar(&duration, "duration", "", "Sync window duration. (e.g. --duration 1h)")
	return command
}

// NewProjectWindowsListCommand returns a new instance of an `argocd proj windows list` command
func NewProjectWindowsListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "list PROJECT",
		Short: "List project sync windows",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			proj, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			printSyncWindows(proj)
		},
	}
	return command
}

// NewProjectWindowsRulesAddCommand returns a new instance of an `argocd proj windows rule add` command
func NewProjectWindowsRulesAddCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		conditions []string
	)
	var command = &cobra.Command{
		Use:   "add PROJECT WINDOW_ID",
		Short: "Add a rule to a sync windows.",
		Long:  "Add a rule. To get WINDOW_ID run \"argocd proj windows list PROJECT\"",
		Example: `
	# Add a rule to a window which will match application name.
	argocd proj windows rules add default 0 --condition "application in web1,db1"

	# Add a rule to a window which will match application name and cluster.
	argocd proj windows rules add default 0 --condition "application in web1,db1" --condition "cluster in in-cluster"

	# Add a rule to a window that will check that a label exists and the namespace matches the prefix "dev-".
	argocd proj windows rules add default 0 --condition "stateful exists" --condition "namespace in dev-*"
`,

		Run: func(c *cobra.Command, args []string) {
			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			projName := args[0]
			windowID, err := strconv.Atoi(args[1])
			errors.CheckError(err)

			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			proj, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			rule, err := generateRule(conditions)
			errors.CheckError(err)

			proj.Spec.SyncWindows[windowID].Rules = append(proj.Spec.SyncWindows[windowID].Rules, *rule)
			errors.CheckError(err)

			_, err = projIf.Update(context.Background(), &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	command.Flags().StringArrayVar(&conditions, "condition", []string{}, "rule conditions")
	return command
}

// NewProjectWindowsRulesDeleteCommand returns a new instance of an `argocd proj windows rule delete` command
func NewProjectWindowsRulesDeleteCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "delete PROJECT WINDOW_ID RULE_ID",
		Short: "Delete a rule from a sync window.",
		Long: "Delete a rule. To get WINDOW_ID and RULE_ID run \"argocd proj windows list PROJECT\" " +
			"and \"argocd proj windows rules list PROJECT WINDOW_ID\"",
		Example: `
	# Delete rule with ID 1 from a window with the ID of 0 in the default project
	argocd proj windows rules delete default 0 1
`,

		Run: func(c *cobra.Command, args []string) {
			if len(args) != 3 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			projName := args[0]
			windowID, err := strconv.Atoi(args[1])
			errors.CheckError(err)
			ruleID, err := strconv.Atoi(args[2])
			errors.CheckError(err)

			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			proj, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			err = proj.Spec.SyncWindows[windowID].DeleteRule(ruleID)
			errors.CheckError(err)

			_, err = projIf.Update(context.Background(), &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	return command
}

// NewProjectWindowsListRulesCommand returns a new instance of an `argocd proj windows rules list` command
func NewProjectWindowsRulesListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "list PROJECT WINDOW_ID",
		Short: "List rules that belong to a sync window.",
		Long:  "List rules. To get WINDOW_ID run \"argocd proj windows list PROJECT\"",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			projName := args[0]
			windowID, err := strconv.Atoi(args[1])
			errors.CheckError(err)

			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			proj, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			printSyncWindowRules(proj, windowID)
		},
	}
	return command
}

// NewProjectWindowsRulesAddConditionCommand returns a new instance of an `argocd proj windows rule add-condition` command
func NewProjectWindowsRulesAddConditionCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		conditions []string
	)
	var command = &cobra.Command{
		Use:   "add-condition PROJECT WINDOW_ID RULE_ID",
		Short: "Add a condition to a rule.",
		Long: "Add a condition to an existing rule. To get WINDOW_ID and RULE_ID run \"argocd proj windows list PROJECT\" " +
			"and \"argocd proj windows rules list PROJECT WINDOW_ID\"",

		Run: func(c *cobra.Command, args []string) {
			if len(args) != 3 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			projName := args[0]
			windowID, err := strconv.Atoi(args[1])
			errors.CheckError(err)
			ruleID, err := strconv.Atoi(args[2])
			errors.CheckError(err)

			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			proj, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			rule, err := generateRule(conditions)
			errors.CheckError(err)

			proj.Spec.SyncWindows[windowID].Rules[ruleID].Conditions = append(proj.Spec.SyncWindows[windowID].Rules[ruleID].Conditions, rule.Conditions...)
			errors.CheckError(err)

			_, err = projIf.Update(context.Background(), &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	command.Flags().StringArrayVar(&conditions, "condition", []string{}, "rule conditions")
	return command
}

// NewProjectWindowsRulesDeleteConditionCommand returns a new instance of an `argocd proj windows rule delete-condition` command
func NewProjectWindowsRulesDeleteConditionCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "delete-condition PROJECT WINDOW_ID RULE_ID CONDITION_ID",
		Short: "Delete a condition from a rule.",
		Long: "Delete a condition from a rule. To get WINDOW_ID, RULE_ID and CONDITION_ID run \"argocd proj windows list PROJECT\" " +
			"and \"argocd proj windows rules list PROJECT WINDOW_ID\"",

		Run: func(c *cobra.Command, args []string) {
			if len(args) != 4 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			projName := args[0]
			windowID, err := strconv.Atoi(args[1])
			errors.CheckError(err)
			ruleID, err := strconv.Atoi(args[2])
			errors.CheckError(err)
			conditionID, err := strconv.Atoi(args[3])
			errors.CheckError(err)

			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			proj, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			err = proj.Spec.SyncWindows[windowID].Rules[ruleID].DeleteCondition(conditionID)
			errors.CheckError(err)

			_, err = projIf.Update(context.Background(), &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	return command
}

// Print table of sync window data
func printSyncWindows(proj *argoappv1.AppProject) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	var fmtStr string
	headers := []interface{}{"ID", "STATUS", "KIND", "SCHEDULE", "DURATION", "RULES", "MANUALSYNC"}
	fmtStr = "%s\t%s\t%s\t%s\t%s\t%s\t%s\n"
	fmt.Fprintf(w, fmtStr, headers...)
	if proj.Spec.SyncWindows.HasWindows() {
		for i, window := range proj.Spec.SyncWindows {
			vals := []interface{}{
				strconv.Itoa(i),
				formatBoolOutput(window.Active()),
				window.Kind,
				window.Schedule,
				window.Duration,
				strconv.Itoa(len(window.Rules) + len(window.Applications) + len(window.Namespaces) + len(window.Clusters)),
				formatManualOutput(window.ManualSync),
			}
			fmt.Fprintf(w, fmtStr, vals...)
		}
	}
	_ = w.Flush()
}

// Print table of sync window data
func printSyncWindowRules(proj *argoappv1.AppProject, index int) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	var fmtStr string
	headers := []interface{}{"ID", "RULE"}
	fmtStr = "%s\t%s\n"
	fmt.Fprintf(w, fmtStr, headers...)
	if proj.Spec.SyncWindows.HasWindows() {
		var conditions []string
		for i, rule := range proj.Spec.SyncWindows[index].Rules {
			for _, comp := range rule.Conditions {
				conditions = append(conditions, formatConditionOutput(comp))
			}
			vals := []interface{}{
				strconv.Itoa(i),
				strings.Join(conditions, " AND "),
			}
			fmt.Fprintf(w, fmtStr, vals...)
			conditions = []string{}
		}
	}
	_ = w.Flush()
}

func formatConditionOutput(c argoappv1.RuleCondition) string {
	var o string
	if c.Key != "" {
		if c.Operator == argoappv1.ConditionOperatorExists {
			o = c.Kind + " " + c.Key + " " + c.Operator
		} else {
			o = c.Kind + " " + c.Key + " " + c.Operator + " (" + strings.Join(c.Values, ", ") + ")"
		}
	} else {
		o = c.Kind + " " + c.Operator + " (" + strings.Join(c.Values, ", ") + ")"
	}
	return o
}

func formatBoolOutput(active bool) string {
	var o string
	if active {
		o = "Active"
	} else {
		o = "Inactive"
	}
	return o
}
func formatManualOutput(active bool) string {
	var o string
	if active {
		o = "Enabled"
	} else {
		o = "Disabled"
	}
	return o
}

func generateRule(conditions []string) (*argoappv1.WindowRule, error) {
	var rule argoappv1.WindowRule
	var values string
	for _, comp := range conditions {
		f := strings.Split(comp, " ")
		kind := f[0]
		operator := f[1]
		if strings.HasSuffix(comp, argoappv1.ConditionOperatorExists) {
			if len(f) != 2 {
				return nil, fmt.Errorf("field mismatch expected 2 got %v", len(f))
			}
		} else if len(f) != 3 {
			return nil, fmt.Errorf("field mismatch expected 3 got %v", len(f))
		} else {
			values = f[2]
		}
		switch operator {
		case argoappv1.ConditionOperatorIn:
			break
		case argoappv1.ConditionOperatorNotIn:
			break
		case argoappv1.ConditionOperatorExists:
			break
		default:
			return nil, fmt.Errorf("operator '%v' not supported", operator)
		}
		nc := argoappv1.RuleCondition{
			Operator: operator,
			Values:   strings.Split(values, ","),
		}
		switch kind {
		case argoappv1.ConditionKindApplication:
			nc.Kind = kind
		case argoappv1.ConditionKindNamespace:
			nc.Kind = kind
		case argoappv1.ConditionKindCluster:
			nc.Kind = kind
		default:
			nc.Kind = argoappv1.ConditionKindLabel
			nc.Key = kind
		}
		rule.Conditions = append(rule.Conditions, nc)
	}
	return &rule, nil
}
