package helm

import (
	"strings"

	"github.com/argoproj/argo-cd/v3/sandbox"
)

func (c Cmd) addRegistryRuntimeOpts(opts *sandbox.SandboxRunOpts, args ...string) {
	numArgs := len(args)
	if numArgs > 0 {
		switch args[0] {
		case "login":
			// args[1] is registry arg
			for idx := 2; idx < numArgs; idx++ {
				switch args[idx] {
				case "--username", "--password":
					idx++
					continue
				case "--ca-file", "--cert-file", "--key-file":
					idx++
					if idx < numArgs {
						opts.ROFiles = append(opts.ROFiles, args[idx])
					}
				case "--insecure":
				default:
				}
			}
		case "logout":
		default:
		}
	}
}

func (c Cmd) addRepoRuntimeOpts(opts *sandbox.SandboxRunOpts, args ...string) {
	numArgs := len(args)
	if numArgs > 0 {
		switch args[0] {
		case "add":
			// args[1] is registry arg
			for idx := 1; idx < numArgs; idx++ {
				switch args[idx] {
				case "--username", "--password":
					idx++
					continue
				case "--cert-file", "--ca-file", "--key-file":
					idx++
					if idx < numArgs {
						opts.ROFiles = append(opts.ROFiles, args[idx])
					}
				case "--insecure-skip-tls-verify", "--pass-credentials":
				default:
				}
			}
		case "logout":
		default:
		}
	}
}

func (c Cmd) addPullRuntimeOpts(opts *sandbox.SandboxRunOpts, args ...string) {
	numArgs := len(args)
	// from one because the first argument is Chart
	idx := 0
	if numArgs > 0 && !strings.HasPrefix(args[0], "--") {
		// oci url
		idx++
	}
	for ; idx < numArgs; idx++ {
		switch args[idx] {
		case "--version", "--username", "--password":
			idx++
			continue
		case "--destination":
			idx++
			if idx < numArgs {
				opts.RWDirs = append(opts.RWDirs, args[idx])
			}
			continue
		case "--ca-file", "--cert-file", "--key-file":
			idx++
			if idx < numArgs {
				opts.ROFiles = append(opts.ROFiles, args[idx])
			}
			continue
		case "--repo":
			idx += 2
			continue
		case "--pass-credentials", "--insecure-skip-tls-verify":
		default:
		}
	}
}

func (c Cmd) addTemplateRuntimeOpts(opts *sandbox.SandboxRunOpts, args ...string) {
	numArgs := len(args)
	// from one because the first argument is Chart
	for idx := 1; idx < numArgs; idx++ {
		switch args[idx] {
		// FIXME: can values be URL?
		// FIXME: array limits

		case "--name-template", "--namespace", "--kube-version", "--set", "--set-string", "--api-versions":
			idx++
			continue
		case "--values":
			idx++
			if idx < numArgs {
				opts.ROFiles = append(opts.ROFiles, args[idx])
			}
		case "--set-file":
			idx++
			if idx < numArgs {
				arg := args[idx]
				parts := strings.SplitN(arg, "=", 2)
				if len(parts) > 1 {
					// FIXME: unescape --set-file value?
					opts.ROFiles = append(opts.ROFiles, parts[1])
				}
			}
		case "--include-crds", "--skip-schema-validation", "--skip-tests":
		default:
		}
	}
}

func (c Cmd) makeSandboxRunOpts(args ...string) *sandbox.SandboxRunOpts {
	sandboxRunOpts := sandbox.SandboxRunOpts{
		RWDirs: []string{
			c.WorkDir,
			c.helmHome,
		},
	}
	if len(args) == 0 {
		return &sandboxRunOpts
	}
	switch args[0] {
	case "template":
		c.addTemplateRuntimeOpts(&sandboxRunOpts, args[1:]...)
	case "registry":
		c.addRegistryRuntimeOpts(&sandboxRunOpts, args[1:]...)
	case "repo":
		c.addRepoRuntimeOpts(&sandboxRunOpts, args[1:]...)
	case "pull":
		c.addPullRuntimeOpts(&sandboxRunOpts, args[1:]...)
	case "dependency":
	case "show":
		// nothing to do
	}

	return &sandboxRunOpts
}
