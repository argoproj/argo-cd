package apiclient

import "github.com/argoproj/argo-cd/v2/cmpserver/apiclient"

func (m *apiclient.ManifestResponse) GetCompiledManifests() []string {
	manifests := make([]string, len(m.Manifests))
	for i, m := range m.Manifests {
		manifests[i] = m.CompiledManifest
	}
	return manifests
}
