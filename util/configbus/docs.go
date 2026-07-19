package configbus

import (
	"fmt"
	"io"
	"sort"
	"text/tabwriter"
)

// WriteReferenceDoc writes a markdown reference of the CURRENT config surface
// (registry descriptors: sources + hot-reload + separator). This documents
// existing config, not a CRD schema.
func WriteReferenceDoc(w io.Writer) error {
	descs := AllDescriptors()
	sort.Slice(descs, func(i, j int) bool { return descs[i].Name() < descs[j].Name() })

	if _, err := fmt.Fprintln(w, "# Argo CD config registry (Phase 0)"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "Auto-generated from `util/configbus` registry descriptors."); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "Describes **current** ConfigMap / env sources only — no CRD paths."); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "Regenerate:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "```bash"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "go run ./hack/config-registry-docs > docs/operator-manual/config-registry.md"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "```"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "| Name | Source | Component | CM key / prefix | Env | Flag | Hot reload | Separator | Secret |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(tw, "| --- | --- | --- | --- | --- | --- | --- | --- | --- |"); err != nil {
		return err
	}
	for _, d := range descs {
		cm := d.CMKeyExact()
		if cm == "" {
			cm = d.CMKeyPrefix()
		}
		if cm == "" {
			cm = "—"
		}
		env := d.EnvVar()
		if env == "" {
			env = "—"
		}
		src := d.SourceConfigMap()
		if src == "" {
			src = "—"
		}
		comp := d.Component()
		if comp == "" {
			comp = "—"
		}
		flag := d.FlagName()
		if flag == "" {
			flag = "—"
		}
		hot := "no"
		if d.HotReload() {
			hot = "yes"
		}
		sec := "no"
		if d.Secret() {
			sec = "yes"
		}
		if _, err := fmt.Fprintf(tw, "| `%s` | `%s` | `%s` | `%s` | `%s` | `%s` | %s | `%s` | %s |\n",
			d.Name(), src, comp, cm, env, flag, hot, d.Separator(), sec); err != nil {
			return err
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "## Notes"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "- Coverage grows as settings migrate onto the registry; see `util/configbus/testdata/unregistered_allowlist.txt`."); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "- Inventory: `go run ./hack/config-inventory -repo-root . -out util/configbus/testdata/inventory.json -allowlist-out util/configbus/testdata/unregistered_allowlist.txt`."); err != nil {
		return err
	}
	return nil
}
