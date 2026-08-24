package configbus

import (
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

func crdFilteredResources(in []appv1.FilteredResource) []settings.FilteredResource {
	if in == nil {
		return nil
	}
	out := make([]settings.FilteredResource, 0, len(in))
	for _, r := range in {
		out = append(out, settings.FilteredResource{
			APIGroups: append([]string(nil), r.APIGroups...),
			Kinds:     append([]string(nil), r.Kinds...),
			Clusters:  append([]string(nil), r.Clusters...),
		})
	}
	return out
}

func crdResourcesFilter(cfg *appv1.ArgoCDConfiguration) (*settings.ResourcesFilter, bool) {
	if cfg == nil || cfg.Spec.Controller == nil || cfg.Spec.Controller.Resource == nil {
		return nil, false
	}
	r := cfg.Spec.Controller.Resource
	if r.Exclusions == nil && r.Inclusions == nil {
		return nil, false
	}
	return &settings.ResourcesFilter{
		ResourceExclusions: crdFilteredResources(r.Exclusions),
		ResourceInclusions: crdFilteredResources(r.Inclusions),
	}, true
}

func crdResourcesFilterExclusions(cfg *appv1.ArgoCDConfiguration) (*settings.ResourcesFilter, bool) {
	if cfg == nil || cfg.Spec.Controller == nil || cfg.Spec.Controller.Resource == nil ||
		cfg.Spec.Controller.Resource.Exclusions == nil {
		return nil, false
	}
	r := cfg.Spec.Controller.Resource
	return &settings.ResourcesFilter{
		ResourceExclusions: crdFilteredResources(r.Exclusions),
		ResourceInclusions: crdFilteredResources(r.Inclusions),
	}, true
}

func crdResourcesFilterInclusions(cfg *appv1.ArgoCDConfiguration) (*settings.ResourcesFilter, bool) {
	if cfg == nil || cfg.Spec.Controller == nil || cfg.Spec.Controller.Resource == nil ||
		cfg.Spec.Controller.Resource.Inclusions == nil {
		return nil, false
	}
	r := cfg.Spec.Controller.Resource
	return &settings.ResourcesFilter{
		ResourceExclusions: crdFilteredResources(r.Exclusions),
		ResourceInclusions: crdFilteredResources(r.Inclusions),
	}, true
}

func crdGlobalProjects(cfg *appv1.ArgoCDConfiguration) ([]settings.GlobalProjectSettings, bool) {
	if cfg == nil || cfg.Spec.Controller == nil || cfg.Spec.Controller.GlobalProjects == nil {
		return nil, false
	}
	out := make([]settings.GlobalProjectSettings, 0, len(cfg.Spec.Controller.GlobalProjects))
	for _, gp := range cfg.Spec.Controller.GlobalProjects {
		entry := settings.GlobalProjectSettings{ProjectName: gp.ProjectName}
		if gp.LabelSelector != nil {
			entry.LabelSelector = *gp.LabelSelector
		}
		out = append(out, entry)
	}
	return out, true
}

func crdDeepLinks(links []appv1.DeepLink) ([]settings.DeepLink, bool) {
	if links == nil {
		return nil, false
	}
	out := make([]settings.DeepLink, 0, len(links))
	for _, l := range links {
		dl := settings.DeepLink{
			URL:   l.URLTemplate,
			Title: l.Title,
		}
		if l.Description != "" {
			desc := l.Description
			dl.Description = &desc
		}
		if l.IconClass != "" {
			ic := l.IconClass
			dl.IconClass = &ic
		}
		if l.ConditionExpr != "" {
			cond := l.ConditionExpr
			dl.Condition = &cond
		}
		out = append(out, dl)
	}
	return out, true
}

func crdApplicationLinks(cfg *appv1.ArgoCDConfiguration) ([]settings.DeepLink, bool) {
	if cfg == nil || cfg.Spec.Server == nil || cfg.Spec.Server.DeepLinks == nil {
		return nil, false
	}
	return crdDeepLinks(cfg.Spec.Server.DeepLinks.Application)
}

func crdProjectLinks(cfg *appv1.ArgoCDConfiguration) ([]settings.DeepLink, bool) {
	if cfg == nil || cfg.Spec.Server == nil || cfg.Spec.Server.DeepLinks == nil {
		return nil, false
	}
	return crdDeepLinks(cfg.Spec.Server.DeepLinks.Project)
}

func crdResourceLinks(cfg *appv1.ArgoCDConfiguration) ([]settings.DeepLink, bool) {
	if cfg == nil || cfg.Spec.Server == nil || cfg.Spec.Server.DeepLinks == nil {
		return nil, false
	}
	return crdDeepLinks(cfg.Spec.Server.DeepLinks.Resource)
}

func crdAccountCapabilities(caps []string) []settings.AccountCapability {
	if len(caps) == 0 {
		return nil
	}
	out := make([]settings.AccountCapability, 0, len(caps))
	for _, c := range caps {
		switch strings.TrimSpace(c) {
		case string(settings.AccountCapabilityLogin):
			out = append(out, settings.AccountCapabilityLogin)
		case string(settings.AccountCapabilityApiKey):
			out = append(out, settings.AccountCapabilityApiKey)
		}
	}
	return out
}

func crdAccounts(cfg *appv1.ArgoCDConfiguration) (map[string]settings.Account, bool) {
	if cfg == nil || cfg.Spec.Server == nil || cfg.Spec.Server.Accounts == nil {
		return nil, false
	}
	out := make(map[string]settings.Account, len(cfg.Spec.Server.Accounts))
	for _, ac := range cfg.Spec.Server.Accounts {
		name := ac.Name
		if name == "" {
			continue
		}
		out[name] = settings.Account{
			// Accounts listed in the CRD are enabled by default (legacy accounts.* / admin.enabled).
			Enabled:      true,
			Capabilities: crdAccountCapabilities(ac.Capabilities),
		}
	}
	return out, true
}

func crdSecretKeyPlaceholder(ref *corev1.SecretKeySelector) string {
	if ref == nil {
		return ""
	}
	if ref.Name != "" && ref.Key != "" {
		return fmt.Sprintf("$secret:%s/%s", ref.Name, ref.Key)
	}
	if ref.Key != "" {
		return fmt.Sprintf("$secret:%s", ref.Key)
	}
	return "$secret:<unset>"
}

func crdDurationString(d *metav1.Duration) string {
	if d == nil {
		return ""
	}
	return d.Duration.String()
}

func crdOIDCLegacyMap(o *appv1.OIDCConfig) map[string]any {
	if o == nil {
		return nil
	}
	m := map[string]any{}
	if o.Name != "" {
		m["name"] = o.Name
	}
	if o.IssuerURL != "" {
		m["issuer"] = o.IssuerURL
	}
	if o.ClientID != "" {
		m["clientID"] = o.ClientID
	}
	if ref := crdSecretKeyPlaceholder(o.ClientSecretRef); ref != "" {
		m["clientSecret"] = ref
	}
	if o.CLIClientID != "" {
		m["cliClientID"] = o.CLIClientID
	}
	if o.UserInfo != nil {
		if o.UserInfo.GroupsEnabled {
			m["enableUserInfoGroups"] = true
		}
		if o.UserInfo.BaseURL != "" {
			m["userInfoBaseURL"] = o.UserInfo.BaseURL
		}
		if o.UserInfo.Path != "" {
			m["userInfoPath"] = o.UserInfo.Path
		}
		if s := crdDurationString(o.UserInfo.CacheExpiration); s != "" {
			m["userInfoCacheExpiration"] = s
		}
	}
	if len(o.RequestedScopes) > 0 {
		m["requestedScopes"] = append([]string(nil), o.RequestedScopes...)
	}
	if len(o.RequestedIDTokenClaims) > 0 {
		claims := map[string]any{}
		for k, c := range o.RequestedIDTokenClaims {
			cm := map[string]any{}
			if c.Essential {
				cm["essential"] = true
			}
			if c.Value != "" {
				cm["value"] = c.Value
			}
			if len(c.Values) > 0 {
				cm["values"] = append([]string(nil), c.Values...)
			}
			claims[k] = cm
		}
		m["requestedIDTokenClaims"] = claims
	}
	if o.LogoutURL != "" {
		m["logoutURL"] = o.LogoutURL
	}
	if o.RootCA != "" {
		m["rootCA"] = o.RootCA
	}
	if o.PKCEAuthenticationEnabled {
		m["enablePKCEAuthentication"] = true
	}
	if o.DomainHint != "" {
		m["domainHint"] = o.DomainHint
	}
	if o.Azure != nil {
		az := map[string]any{}
		if o.Azure.UseWorkloadIdentity {
			az["useWorkloadIdentity"] = true
		}
		if o.Azure.GraphAPIEndpointURL != "" {
			az["graphApiEndpoint"] = o.Azure.GraphAPIEndpointURL
		}
		if o.Azure.UserGroupOverageClaim != nil {
			if o.Azure.UserGroupOverageClaim.Enabled {
				az["enableUserGroupOverageClaim"] = true
			}
			if s := crdDurationString(o.Azure.UserGroupOverageClaim.CacheExpiration); s != "" {
				az["userGroupOverageClaimCacheExpiration"] = s
			}
		}
		if len(az) > 0 {
			m["azure"] = az
		}
	}
	if s := crdDurationString(o.RefreshTokenThreshold); s != "" {
		m["refreshTokenThreshold"] = s
	}
	if len(o.AllowedAudiences) > 0 {
		m["allowedAudiences"] = append([]string(nil), o.AllowedAudiences...)
	}
	if o.SkipAudienceCheckWhenTokenHasNoAudience {
		m["skipAudienceCheckWhenTokenHasNoAudience"] = true
	}
	return m
}

func crdOIDCConfigYAML(cfg *appv1.ArgoCDConfiguration) (string, bool, error) {
	if cfg == nil || cfg.Spec.Server == nil || cfg.Spec.Server.OIDC == nil {
		return "", false, nil
	}
	m := crdOIDCLegacyMap(cfg.Spec.Server.OIDC)
	if len(m) == 0 {
		return "", false, nil
	}
	b, err := yaml.Marshal(m)
	if err != nil {
		return "", false, fmt.Errorf("marshal oidc config: %w", err)
	}
	return string(b), true, nil
}

func rawExtensionToAny(raw runtime.RawExtension) (any, error) {
	if len(raw.Raw) == 0 {
		return nil, nil
	}
	var v any
	if err := json.Unmarshal(raw.Raw, &v); err != nil {
		return nil, err
	}
	return v, nil
}

func crdDexConfigYAML(cfg *appv1.ArgoCDConfiguration) (string, bool, error) {
	if cfg == nil || cfg.Spec.Server == nil || cfg.Spec.Server.Dex == nil {
		return "", false, nil
	}
	d := cfg.Spec.Server.Dex
	m := map[string]any{}
	if len(d.Connectors) > 0 {
		connectors := make([]map[string]any, 0, len(d.Connectors))
		for _, c := range d.Connectors {
			entry := map[string]any{
				"type": c.Type,
				"id":   c.ID,
				"name": c.Name,
			}
			if cfgVal, err := rawExtensionToAny(c.Config); err != nil {
				return "", false, fmt.Errorf("marshal dex connector %q config: %w", c.ID, err)
			} else if cfgVal != nil {
				entry["config"] = cfgVal
			}
			connectors = append(connectors, entry)
		}
		m["connectors"] = connectors
	}
	if len(d.StaticClients) > 0 {
		clients := make([]any, 0, len(d.StaticClients))
		for _, sc := range d.StaticClients {
			v, err := rawExtensionToAny(sc)
			if err != nil {
				return "", false, fmt.Errorf("marshal dex staticClient: %w", err)
			}
			if v != nil {
				clients = append(clients, v)
			}
		}
		if len(clients) > 0 {
			m["staticClients"] = clients
		}
	}
	if d.Extra != nil {
		extra, err := rawExtensionToAny(*d.Extra)
		if err != nil {
			return "", false, fmt.Errorf("marshal dex extra config: %w", err)
		}
		if extraMap, ok := extra.(map[string]any); ok {
			for k, v := range extraMap {
				m[k] = v
			}
		}
	}
	if len(m) == 0 {
		return "", false, nil
	}
	b, err := yaml.Marshal(m)
	if err != nil {
		return "", false, fmt.Errorf("marshal dex config: %w", err)
	}
	return string(b), true, nil
}

func crdKustomizeOptionsFromCRD(cfg *appv1.ArgoCDConfiguration) (*appv1.KustomizeOptions, bool) {
	if cfg == nil || cfg.Spec.RepoServer == nil || cfg.Spec.RepoServer.Kustomize == nil {
		return nil, false
	}
	k := cfg.Spec.RepoServer.Kustomize
	if k.Versions == nil && k.BuildOptions == "" {
		return nil, false
	}
	out := &appv1.KustomizeOptions{BuildOptions: k.BuildOptions}
	if k.Versions != nil {
		out.Versions = append([]appv1.KustomizeVersion(nil), k.Versions...)
	}
	return out, true
}

func crdKustomizeVersions(cfg *appv1.ArgoCDConfiguration) (*appv1.KustomizeOptions, bool) {
	if cfg == nil || cfg.Spec.RepoServer == nil || cfg.Spec.RepoServer.Kustomize == nil ||
		cfg.Spec.RepoServer.Kustomize.Versions == nil {
		return nil, false
	}
	return crdKustomizeOptionsFromCRD(cfg)
}

func crdHelpDownload(cfg *appv1.ArgoCDConfiguration) (*settings.Help, bool) {
	if cfg == nil || cfg.Spec.Server == nil || cfg.Spec.Server.Help == nil ||
		cfg.Spec.Server.Help.BinaryURLs == nil {
		return nil, false
	}
	return &settings.Help{BinaryURLs: copyStringMap(cfg.Spec.Server.Help.BinaryURLs)}, true
}

func copyStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func crdExtensionBackendMap(ext appv1.ExtensionConfig) map[string]any {
	m := map[string]any{}
	if ext.ConnectionTimeout != nil && ext.ConnectionTimeout.Duration > 0 {
		m["connectionTimeout"] = ext.ConnectionTimeout.Duration.String()
	}
	if ext.KeepAlive != nil && ext.KeepAlive.Duration > 0 {
		m["keepAlive"] = ext.KeepAlive.Duration.String()
	}
	if ext.IdleConnectionTimeout != nil && ext.IdleConnectionTimeout.Duration > 0 {
		m["idleConnectionTimeout"] = ext.IdleConnectionTimeout.Duration.String()
	}
	if ext.MaxIdleConnections > 0 {
		m["maxIdleConnections"] = ext.MaxIdleConnections
	}
	if len(ext.Services) > 0 {
		services := make([]map[string]any, 0, len(ext.Services))
		for _, svc := range ext.Services {
			sm := map[string]any{"url": svc.URL}
			if svc.Cluster != nil {
				cluster := map[string]any{}
				if svc.Cluster.ServerURL != "" {
					cluster["server"] = svc.Cluster.ServerURL
				}
				if svc.Cluster.Name != "" {
					cluster["name"] = svc.Cluster.Name
				}
				if len(cluster) > 0 {
					sm["cluster"] = cluster
				}
			}
			if len(svc.Headers) > 0 {
				headers := make([]map[string]string, 0, len(svc.Headers))
				for _, h := range svc.Headers {
					headers = append(headers, map[string]string{"name": h.Name, "value": h.Value})
				}
				sm["headers"] = headers
			}
			services = append(services, sm)
		}
		m["services"] = services
	}
	return m
}

func crdExtensionConfig(cfg *appv1.ArgoCDConfiguration) (map[string]string, bool, error) {
	if cfg == nil || cfg.Spec.Server == nil || cfg.Spec.Server.Extensions == nil {
		return nil, false, nil
	}
	out := make(map[string]string, len(cfg.Spec.Server.Extensions))
	for _, ext := range cfg.Spec.Server.Extensions {
		if ext.Name == "" {
			continue
		}
		be := crdExtensionBackendMap(ext)
		b, err := yaml.Marshal(be)
		if err != nil {
			return nil, false, fmt.Errorf("marshal extension %q: %w", ext.Name, err)
		}
		out[ext.Name] = string(b)
	}
	return out, true, nil
}
