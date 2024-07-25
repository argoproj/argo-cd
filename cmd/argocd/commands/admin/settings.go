package admin

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	healthutil "github.com/argoproj/gitops-engine/pkg/health"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/argo/normalizers"
	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/lua"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

type settingsOpts struct {
	argocdCMPath        string
	argocdSecretPath    string
	loadClusterSettings bool
	clientConfig        clientcmd.ClientConfig
}

type commandContext interface {
	createSettingsManager(context.Context) (*settings.SettingsManager, error)
}

func collectLogs(callback func()) string {
	log.SetLevel(log.DebugLevel)
	out := bytes.Buffer{}
	log.SetOutput(&out)
	defer log.SetLevel(log.FatalLevel)
	callback()
	return out.String()
}

func setSettingsMeta(obj v1.Object) {
	obj.SetNamespace("default")
	labels := obj.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["app.kubernetes.io/part-of"] = "argocd"
	obj.SetLabels(labels)
}

func (opts *settingsOpts) createSettingsManager(ctx context.Context) (*settings.SettingsManager, error) {
	var argocdCM *corev1.ConfigMap
	if opts.argocdCMPath == "" && !opts.loadClusterSettings {
		return nil, fmt.Errorf("either --argocd-cm-path must be provided or --load-cluster-settings must be set to true")
	} else if opts.argocdCMPath == "" {
		realClientset, ns, err := opts.getK8sClient()
		if err != nil {
			return nil, err
		}

		argocdCM, err = realClientset.CoreV1().ConfigMaps(ns).Get(ctx, common.ArgoCDConfigMapName, v1.GetOptions{})
		if err != nil {
			return nil, err
		}
	} else {
		data, err := os.ReadFile(opts.argocdCMPath)
		if err != nil {
			return nil, err
		}
		err = yaml.Unmarshal(data, &argocdCM)
		if err != nil {
			return nil, err
		}
	}
	setSettingsMeta(argocdCM)

	var argocdSecret *corev1.Secret
	if opts.argocdSecretPath != "" {
		data, err := os.ReadFile(opts.argocdSecretPath)
		if err != nil {
			return nil, err
		}
		err = yaml.Unmarshal(data, &argocdSecret)
		if err != nil {
			return nil, err
		}
		setSettingsMeta(argocdSecret)
	} else if opts.loadClusterSettings {
		realClientset, ns, err := opts.getK8sClient()
		if err != nil {
			return nil, err
		}
		argocdSecret, err = realClientset.CoreV1().Secrets(ns).Get(ctx, common.ArgoCDSecretName, v1.GetOptions{})
		if err != nil {
			return nil, err
		}
	} else {
		argocdSecret = &corev1.Secret{
			ObjectMeta: v1.ObjectMeta{
				Name: common.ArgoCDSecretName,
			},
			Data: map[string][]byte{
				"admin.password":   []byte("test"),
				"server.secretkey": []byte("test"),
			},
		}
	}
	setSettingsMeta(argocdSecret)
	clientset := fake.NewSimpleClientset(argocdSecret, argocdCM)

	manager := settings.NewSettingsManager(ctx, clientset, "default")
	errors.CheckError(manager.ResyncInformers())

	return manager, nil
}

func (opts *settingsOpts) getK8sClient() (*kubernetes.Clientset, string, error) {
	namespace, _, err := opts.clientConfig.Namespace()
	if err != nil {
		return nil, "", err
	}

	restConfig, err := opts.clientConfig.ClientConfig()
	if err != nil {
		return nil, "", err
	}

	realClientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, "", err
	}
	return realClientset, namespace, nil
}

func NewSettingsCommand() *cobra.Command {
	var opts settingsOpts

	command := &cobra.Command{
		Use:   "settings",
		Short: "Provides set of commands for settings validation and troubleshooting",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
	}
	log.SetLevel(log.FatalLevel)

	command.AddCommand(NewValidateSettingsCommand(&opts))
	command.AddCommand(NewResourceOverridesCommand(&opts))
	command.AddCommand(NewRBACCommand(&opts))

	opts.clientConfig = cli.AddKubectlFlagsToCmd(command)
	command.PersistentFlags().StringVar(&opts.argocdCMPath, "argocd-cm-path", "", "Path to local argocd-cm.yaml file")
	command.PersistentFlags().StringVar(&opts.argocdSecretPath, "argocd-secret-path", "", "Path to local argocd-secret.yaml file")
	command.PersistentFlags().BoolVar(&opts.loadClusterSettings, "load-cluster-settings", false,
		"Indicates that config map and secret should be loaded from cluster unless local file path is provided")
	return command
}

type settingValidator func(manager *settings.SettingsManager) (string, error)

func joinValidators(validators ...settingValidator) settingValidator {
	return func(manager *settings.SettingsManager) (string, error) {
		var errorStrs []string
		var summaries []string
		for i := range validators {
			summary, err := validators[i](manager)
			if err != nil {
				errorStrs = append(errorStrs, err.Error())
			}
			if summary != "" {
				summaries = append(summaries, summary)
			}
		}
		if len(errorStrs) > 0 {
			return "", fmt.Errorf("%s", strings.Join(errorStrs, "\n"))
		}
		return strings.Join(summaries, "\n"), nil
	}
}

var validatorsByGroup = map[string]settingValidator{
	"general": joinValidators(func(manager *settings.SettingsManager) (string, error) {
		general, err := manager.GetSettings()
		if err != nil {
			return "", err
		}
		ssoProvider := ""
		if general.DexConfig != "" {
			if _, err := settings.UnmarshalDexConfig(general.DexConfig); err != nil {
				return "", fmt.Errorf("invalid dex.config: %w", err)
			}
			ssoProvider = "Dex"
		} else if general.OIDCConfigRAW != "" {
			if err := settings.ValidateOIDCConfig(general.OIDCConfigRAW); err != nil {
				return "", fmt.Errorf("invalid oidc.config: %w", err)
			}
			ssoProvider = "OIDC"
		}
		var summary string
		if ssoProvider != "" {
			summary = fmt.Sprintf("%s is configured", ssoProvider)
			if general.URL == "" {
				summary = summary + " ('url' field is missing)"
			}
		} else if ssoProvider != "" && general.URL != "" {
		} else {
			summary = "SSO is not configured"
		}
		return summary, nil
	}, func(manager *settings.SettingsManager) (string, error) {
		_, err := manager.GetAppInstanceLabelKey()
		return "", err
	}, func(manager *settings.SettingsManager) (string, error) {
		_, err := manager.GetHelp()
		return "", err
	}, func(manager *settings.SettingsManager) (string, error) {
		_, err := manager.GetGoogleAnalytics()
		return "", err
	}),
	"kustomize": func(manager *settings.SettingsManager) (string, error) {
		opts, err := manager.GetKustomizeSettings()
		if err != nil {
			return "", err
		}
		summary := "default options"
		if opts.BuildOptions != "" {
			summary = opts.BuildOptions
		}
		if len(opts.Versions) > 0 {
			summary = fmt.Sprintf("%s (%d versions)", summary, len(opts.Versions))
		}
		return summary, err
	},
	"repositories": joinValidators(func(manager *settings.SettingsManager) (string, error) {
		repos, err := manager.GetRepositories()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d repositories", len(repos)), nil
	}, func(manager *settings.SettingsManager) (string, error) {
		creds, err := manager.GetRepositoryCredentials()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d repository credentials", len(creds)), nil
	}),
	"accounts": func(manager *settings.SettingsManager) (string, error) {
		accounts, err := manager.GetAccounts()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d accounts", len(accounts)), nil
	},
	"resource-overrides": func(manager *settings.SettingsManager) (string, error) {
		overrides, err := manager.GetResourceOverrides()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d resource overrides", len(overrides)), nil
	},
}

func NewValidateSettingsCommand(cmdCtx commandContext) *cobra.Command {
	var groups []string

	var allGroups []string
	for k := range validatorsByGroup {
		allGroups = append(allGroups, k)
	}
	sort.Slice(allGroups, func(i, j int) bool {
		return allGroups[i] < allGroups[j]
	})

	command := &cobra.Command{
		Use:   "validate",
		Short: "Validate settings",
		Long:  "Validates settings specified in 'argocd-cm' ConfigMap and 'argocd-secret' Secret",
		Example: `
#Validates all settings in the specified YAML file
argocd admin settings validate --argocd-cm-path ./argocd-cm.yaml

#Validates accounts and plugins settings in Kubernetes cluster of current kubeconfig context
argocd admin settings validate --group accounts --group plugins --load-cluster-settings`,
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			settingsManager, err := cmdCtx.createSettingsManager(ctx)
			errors.CheckError(err)

			if len(groups) == 0 {
				groups = allGroups
			}
			for i, group := range groups {
				validator := validatorsByGroup[group]

				logs := collectLogs(func() {
					summary, err := validator(settingsManager)

					if err != nil {
						_, _ = fmt.Fprintf(os.Stdout, "❌ %s\n", group)
						_, _ = fmt.Fprintf(os.Stdout, "%s\n", err.Error())
					} else {
						_, _ = fmt.Fprintf(os.Stdout, "✅ %s\n", group)
						if summary != "" {
							_, _ = fmt.Fprintf(os.Stdout, "%s\n", summary)
						}
					}
				})
				if logs != "" {
					_, _ = fmt.Fprintf(os.Stdout, "%s\n", logs)
				}
				if i != len(groups)-1 {
					_, _ = fmt.Fprintf(os.Stdout, "\n")
				}
			}
		},
	}

	command.Flags().StringArrayVar(&groups, "group", nil, fmt.Sprintf(
		"Optional list of setting groups that have to be validated ( one of: %s)", strings.Join(allGroups, ", ")))

	return command
}

func NewResourceOverridesCommand(cmdCtx commandContext) *cobra.Command {
	command := &cobra.Command{
		Use:   "resource-overrides",
		Short: "Troubleshoot resource overrides",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
	}
	command.AddCommand(NewResourceIgnoreDifferencesCommand(cmdCtx))
	command.AddCommand(NewResourceIgnoreResourceUpdatesCommand(cmdCtx))
	command.AddCommand(NewResourceActionListCommand(cmdCtx))
	command.AddCommand(NewResourceActionRunCommand(cmdCtx))
	command.AddCommand(NewResourceHealthCommand(cmdCtx))
	return command
}

func executeResourceOverrideCommand(ctx context.Context, cmdCtx commandContext, args []string, callback func(res unstructured.Unstructured, override v1alpha1.ResourceOverride, overrides map[string]v1alpha1.ResourceOverride)) {
	data, err := os.ReadFile(args[0])
	errors.CheckError(err)

	res := unstructured.Unstructured{}
	errors.CheckError(yaml.Unmarshal(data, &res))

	settingsManager, err := cmdCtx.createSettingsManager(ctx)
	errors.CheckError(err)

	overrides, err := settingsManager.GetResourceOverrides()
	errors.CheckError(err)
	gvk := res.GroupVersionKind()
	key := gvk.Kind
	if gvk.Group != "" {
		key = fmt.Sprintf("%s/%s", gvk.Group, gvk.Kind)
	}
	override := overrides[key]
	callback(res, override, overrides)
}

func executeIgnoreResourceUpdatesOverrideCommand(ctx context.Context, cmdCtx commandContext, args []string, callback func(res unstructured.Unstructured, override v1alpha1.ResourceOverride, overrides map[string]v1alpha1.ResourceOverride)) {
	data, err := os.ReadFile(args[0])
	errors.CheckError(err)

	res := unstructured.Unstructured{}
	errors.CheckError(yaml.Unmarshal(data, &res))

	settingsManager, err := cmdCtx.createSettingsManager(ctx)
	errors.CheckError(err)

	overrides, err := settingsManager.GetIgnoreResourceUpdatesOverrides()
	errors.CheckError(err)
	gvk := res.GroupVersionKind()
	key := gvk.Kind
	if gvk.Group != "" {
		key = fmt.Sprintf("%s/%s", gvk.Group, gvk.Kind)
	}
	override, hasOverride := overrides[key]
	if !hasOverride {
		_, _ = fmt.Printf("No overrides configured for '%s/%s'\n", gvk.Group, gvk.Kind)
		return
	}
	callback(res, override, overrides)
}

func NewResourceIgnoreDifferencesCommand(cmdCtx commandContext) *cobra.Command {
	command := &cobra.Command{
		Use:   "ignore-differences RESOURCE_YAML_PATH",
		Short: "Renders fields excluded from diffing",
		Long:  "Renders ignored fields using the 'ignoreDifferences' setting specified in the 'resource.customizations' field of 'argocd-cm' ConfigMap",
		Example: `
argocd admin settings resource-overrides ignore-differences ./deploy.yaml --argocd-cm-path ./argocd-cm.yaml`,
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) < 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			executeResourceOverrideCommand(ctx, cmdCtx, args, func(res unstructured.Unstructured, override v1alpha1.ResourceOverride, overrides map[string]v1alpha1.ResourceOverride) {
				gvk := res.GroupVersionKind()
				if len(override.IgnoreDifferences.JSONPointers) == 0 && len(override.IgnoreDifferences.JQPathExpressions) == 0 {
					_, _ = fmt.Printf("Ignore differences are not configured for '%s/%s'\n", gvk.Group, gvk.Kind)
					return
				}

				// This normalizer won't verify 'managedFieldsManagers' ignore difference
				// configurations. This requires access to live resources which is not the
				// purpose of this command. This will just apply jsonPointers and
				// jqPathExpressions configurations.
				normalizer, err := normalizers.NewIgnoreNormalizer(nil, overrides, normalizers.IgnoreNormalizerOpts{})
				errors.CheckError(err)

				normalizedRes := res.DeepCopy()
				logs := collectLogs(func() {
					errors.CheckError(normalizer.Normalize(normalizedRes))
				})
				if logs != "" {
					_, _ = fmt.Println(logs)
				}

				if reflect.DeepEqual(&res, normalizedRes) {
					_, _ = fmt.Printf("No fields are ignored by ignoreDifferences settings: \n%s\n", override.IgnoreDifferences)
					return
				}

				_, _ = fmt.Printf("Following fields are ignored:\n\n")
				_ = cli.PrintDiff(res.GetName(), &res, normalizedRes)
			})
		},
	}
	return command
}

func NewResourceIgnoreResourceUpdatesCommand(cmdCtx commandContext) *cobra.Command {
	var ignoreNormalizerOpts normalizers.IgnoreNormalizerOpts
	command := &cobra.Command{
		Use:   "ignore-resource-updates RESOURCE_YAML_PATH",
		Short: "Renders fields excluded from resource updates",
		Long:  "Renders ignored fields using the 'ignoreResourceUpdates' setting specified in the 'resource.customizations' field of 'argocd-cm' ConfigMap",
		Example: `
argocd admin settings resource-overrides ignore-resource-updates ./deploy.yaml --argocd-cm-path ./argocd-cm.yaml`,
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) < 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			executeIgnoreResourceUpdatesOverrideCommand(ctx, cmdCtx, args, func(res unstructured.Unstructured, override v1alpha1.ResourceOverride, overrides map[string]v1alpha1.ResourceOverride) {
				gvk := res.GroupVersionKind()
				if len(override.IgnoreResourceUpdates.JSONPointers) == 0 && len(override.IgnoreResourceUpdates.JQPathExpressions) == 0 {
					_, _ = fmt.Printf("Ignore resource updates are not configured for '%s/%s'\n", gvk.Group, gvk.Kind)
					return
				}

				normalizer, err := normalizers.NewIgnoreNormalizer(nil, overrides, ignoreNormalizerOpts)
				errors.CheckError(err)

				normalizedRes := res.DeepCopy()
				logs := collectLogs(func() {
					errors.CheckError(normalizer.Normalize(normalizedRes))
				})
				if logs != "" {
					_, _ = fmt.Println(logs)
				}

				if reflect.DeepEqual(&res, normalizedRes) {
					_, _ = fmt.Printf("No fields are ignored by ignoreResourceUpdates settings: \n%s\n", override.IgnoreResourceUpdates)
					return
				}

				_, _ = fmt.Printf("Following fields are ignored:\n\n")
				_ = cli.PrintDiff(res.GetName(), &res, normalizedRes)
			})
		},
	}
	command.Flags().DurationVar(&ignoreNormalizerOpts.JQExecutionTimeout, "ignore-normalizer-jq-execution-timeout", normalizers.DefaultJQExecutionTimeout, "Set ignore normalizer JQ execution timeout")
	return command
}

func NewResourceHealthCommand(cmdCtx commandContext) *cobra.Command {
	command := &cobra.Command{
		Use:   "health RESOURCE_YAML_PATH",
		Short: "Assess resource health",
		Long:  "Assess resource health using the lua script configured in the 'resource.customizations' field of 'argocd-cm' ConfigMap",
		Example: `
argocd admin settings resource-overrides health ./deploy.yaml --argocd-cm-path ./argocd-cm.yaml`,
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) < 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			executeResourceOverrideCommand(ctx, cmdCtx, args, func(res unstructured.Unstructured, override v1alpha1.ResourceOverride, overrides map[string]v1alpha1.ResourceOverride) {
				gvk := res.GroupVersionKind()
				resHealth, err := healthutil.GetResourceHealth(&res, lua.ResourceHealthOverrides(overrides))

				if err != nil {
					errors.CheckError(err)
				} else if resHealth == nil {
					fmt.Printf("Health script is not configured for '%s/%s'\n", gvk.Group, gvk.Kind)
				} else {
					_, _ = fmt.Printf("STATUS: %s\n", resHealth.Status)
					_, _ = fmt.Printf("MESSAGE: %s\n", resHealth.Message)
				}
			})
		},
	}
	return command
}

func NewResourceActionListCommand(cmdCtx commandContext) *cobra.Command {
	command := &cobra.Command{
		Use:   "list-actions RESOURCE_YAML_PATH",
		Short: "List available resource actions",
		Long:  "List actions available for given resource action using the lua scripts configured in the 'resource.customizations' field of 'argocd-cm' ConfigMap and outputs updated fields",
		Example: `
argocd admin settings resource-overrides action list /tmp/deploy.yaml --argocd-cm-path ./argocd-cm.yaml`,
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) < 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			executeResourceOverrideCommand(ctx, cmdCtx, args, func(res unstructured.Unstructured, override v1alpha1.ResourceOverride, overrides map[string]v1alpha1.ResourceOverride) {
				gvk := res.GroupVersionKind()
				if override.Actions == "" {
					_, _ = fmt.Printf("Actions are not configured for '%s/%s'\n", gvk.Group, gvk.Kind)
					return
				}

				luaVM := lua.VM{ResourceOverrides: overrides}
				discoveryScript, err := luaVM.GetResourceActionDiscovery(&res)
				errors.CheckError(err)

				availableActions, err := luaVM.ExecuteResourceActionDiscovery(&res, discoveryScript)
				errors.CheckError(err)
				sort.Slice(availableActions, func(i, j int) bool {
					return availableActions[i].Name < availableActions[j].Name
				})

				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				_, _ = fmt.Fprintf(w, "NAME\tDISABLED\n")
				for _, action := range availableActions {
					_, _ = fmt.Fprintf(w, "%s\t%s\n", action.Name, strconv.FormatBool(action.Disabled))
				}
				_ = w.Flush()
			})
		},
	}
	return command
}

func NewResourceActionRunCommand(cmdCtx commandContext) *cobra.Command {
	command := &cobra.Command{
		Use:     "run-action RESOURCE_YAML_PATH ACTION",
		Aliases: []string{"action"},
		Short:   "Executes resource action",
		Long:    "Executes resource action using the lua script configured in the 'resource.customizations' field of 'argocd-cm' ConfigMap and outputs updated fields",
		Example: `
argocd admin settings resource-overrides action run /tmp/deploy.yaml restart --argocd-cm-path ./argocd-cm.yaml`,
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) < 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			action := args[1]

			executeResourceOverrideCommand(ctx, cmdCtx, args, func(res unstructured.Unstructured, override v1alpha1.ResourceOverride, overrides map[string]v1alpha1.ResourceOverride) {
				gvk := res.GroupVersionKind()
				if override.Actions == "" {
					_, _ = fmt.Printf("Actions are not configured for '%s/%s'\n", gvk.Group, gvk.Kind)
					return
				}

				luaVM := lua.VM{ResourceOverrides: overrides}
				action, err := luaVM.GetResourceAction(&res, action)
				errors.CheckError(err)

				modifiedRes, err := luaVM.ExecuteResourceAction(&res, action.ActionLua)
				errors.CheckError(err)

				for _, impactedResource := range modifiedRes {
					result := impactedResource.UnstructuredObj
					switch impactedResource.K8SOperation {
					// No default case since a not supported operation would have failed upon unmarshaling earlier
					case lua.PatchOperation:
						if reflect.DeepEqual(&res, modifiedRes) {
							_, _ = fmt.Printf("No fields had been changed by action: \n%s\n", action.Name)
							return
						}

						_, _ = fmt.Printf("Following fields have been changed:\n\n")
						_ = cli.PrintDiff(res.GetName(), &res, result)
					case lua.CreateOperation:
						yamlBytes, err := yaml.Marshal(impactedResource.UnstructuredObj)
						errors.CheckError(err)
						fmt.Println("Following resource was created:")
						fmt.Println(bytes.NewBuffer(yamlBytes).String())
					}
				}
			})
		},
	}
	return command
}
