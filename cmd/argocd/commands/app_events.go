package commands

import (
	"context"
	stderrors "errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v3/cmd/argocd/commands/headless"
	argocdclient "github.com/argoproj/argo-cd/v3/pkg/apiclient"
	applicationpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	eventspkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/events"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/argo"
	"github.com/argoproj/argo-cd/v3/util/cli"
	"github.com/argoproj/argo-cd/v3/util/errors"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
)

// resourceEventsFilter selects a resource of an application whose events should be listed
type resourceEventsFilter struct {
	kind      string
	group     string
	namespace string
	name      string
}

// isSet reports whether the filter selects a resource. Without a resource, the events of the application itself are listed.
func (f resourceEventsFilter) isSet() bool {
	return f.kind != "" || f.name != "" || f.group != "" || f.namespace != ""
}

func (f resourceEventsFilter) validate() error {
	if f.isSet() && (f.kind == "" || f.name == "") {
		return stderrors.New("--kind and --resource-name must be specified together to list the events of a resource")
	}
	return nil
}

// NewApplicationEventsCommand returns a new instance of an `argocd app events` command
func NewApplicationEventsCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		appNamespace string
		filter       resourceEventsFilter
		output       string
	)
	command := &cobra.Command{
		Use:   "events APPNAME",
		Short: "List the events of an application or of one of its resources",
		Long:  "List the events of an application or of one of its resources. Without --kind and --resource-name, the events of the Application object are listed. With them, the events of the matching resource in the application are listed.",
		Example: `  # List the events of the application
  argocd app events my-app

  # List the events of a resource in the application
  argocd app events my-app --kind Deployment --resource-name guestbook-ui

  # Select the resource by group and namespace when several resources match
  argocd app events my-app --kind Deployment --group apps --namespace default --resource-name guestbook-ui

  # Print the events in JSON format
  argocd app events my-app -o json`,
		Run: cli.WithSignalContext(func(c *cobra.Command, args []string, _ context.CancelFunc) {
			ctx := c.Context()

			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			errors.CheckError(filter.validate())
			if output != "wide" && output != "json" && output != "yaml" {
				errors.CheckError(fmt.Errorf("unknown output format: %s", output))
			}

			appName, appNs := argo.ParseFromQualifiedName(args[0], appNamespace)

			conn, appIf := headless.NewClientOrDie(clientOpts, c).NewApplicationClientOrDieWithContext(ctx)
			defer utilio.Close(conn)

			events, err := listApplicationEvents(ctx, appIf, appName, appNs, filter)
			errors.CheckError(err)

			if output == "wide" {
				printEventsTable(os.Stdout, events)
				return
			}
			errors.CheckError(PrintResourceList(events, output, false))
		}),
	}
	command.Flags().StringVarP(&appNamespace, "app-namespace", "N", "", "Namespace of the application")
	command.Flags().StringVar(&filter.kind, "kind", "", "Kind of the resource")
	command.Flags().StringVar(&filter.group, "group", "", "Group of the resource")
	command.Flags().StringVar(&filter.namespace, "namespace", "", "Namespace of the resource")
	command.Flags().StringVar(&filter.name, "resource-name", "", "Name of the resource")
	command.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: wide|json|yaml")
	return command
}

// listApplicationEvents returns the events of the application, or of the resource selected by the filter, sorted from the oldest to the most recent
func listApplicationEvents(ctx context.Context, appIf applicationpkg.ApplicationServiceClient, appName, appNs string, filter resourceEventsFilter) ([]eventspkg.Event, error) {
	query := &applicationpkg.ApplicationResourceEventsQuery{
		Name:         &appName,
		AppNamespace: &appNs,
	}
	if filter.isSet() {
		tree, err := appIf.ResourceTree(ctx, &applicationpkg.ResourcesQuery{
			ApplicationName: &appName,
			AppNamespace:    &appNs,
		})
		if err != nil {
			return nil, fmt.Errorf("error getting resource tree of application %s: %w", appName, err)
		}
		node, err := findResourceNode(tree, appName, filter)
		if err != nil {
			return nil, err
		}
		query.ResourceName = &node.Name
		query.ResourceNamespace = &node.Namespace
		query.ResourceUID = &node.UID
	}

	list, err := appIf.ListResourceEvents(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error listing events: %w", err)
	}
	events := list.Items
	sort.SliceStable(events, func(i, j int) bool {
		return eventLastSeen(events[i]).Before(eventLastSeen(events[j]))
	})
	return events, nil
}

// findResourceNode returns the single resource of the application tree that matches the filter
func findResourceNode(tree *v1alpha1.ApplicationTree, appName string, filter resourceEventsFilter) (*v1alpha1.ResourceNode, error) {
	var matches []v1alpha1.ResourceNode
	for _, nodes := range [][]v1alpha1.ResourceNode{tree.Nodes, tree.OrphanedNodes} {
		for _, n := range nodes {
			if n.Kind != filter.kind || n.Name != filter.name {
				continue
			}
			if (filter.group != "" && n.Group != filter.group) || (filter.namespace != "" && n.Namespace != filter.namespace) {
				continue
			}
			matches = append(matches, n)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("resource %s/%s not found in application %s", filter.kind, filter.name, appName)
	case 1:
		return &matches[0], nil
	default:
		return nil, fmt.Errorf("%d resources match %s/%s in application %s, use --group or --namespace to select one", len(matches), filter.kind, filter.name, appName)
	}
}

// eventLastSeen returns when the event was last observed. Events created through the events.k8s.io API only set eventTime.
func eventLastSeen(e eventspkg.Event) time.Time {
	if !e.LastTimestamp.IsZero() {
		return e.LastTimestamp.Time
	}
	return e.EventTime.Time
}

func printEventsTable(out io.Writer, events []eventspkg.Event) {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, "LAST SEEN\tTYPE\tREASON\tOBJECT\tCOUNT\tMESSAGE\n")
	for _, e := range events {
		lastSeen := "-"
		if t := eventLastSeen(e); !t.IsZero() {
			lastSeen = t.UTC().Format(time.RFC3339)
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s/%s\t%s\t%s\n",
			lastSeen,
			e.Type,
			e.Reason,
			e.InvolvedObject.Kind,
			e.InvolvedObject.Name,
			strconv.Itoa(int(e.Count)),
			strings.ReplaceAll(e.Message, "\n", " "),
		)
	}
	_ = w.Flush()
}
