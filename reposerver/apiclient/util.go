package apiclient

func (m *ManifestResponse) GetCompiledManifests() []string {
	manifests := make([]string, len(m.Manifests))
	for i, m := range m.Manifests {
		manifests[i] = m.CompiledManifest
	}
	return manifests
}
