package apiclient

func (q *ManifestRequest) GetValuesFileSchemes() []string {
	if q.HelmOptions == nil {
		return nil
	}
	return q.HelmOptions.ValuesFileSchemes
}

func (q *RepoServerAppDetailsQuery) GetValuesFileSchemes() []string {
	if q.HelmOptions == nil {
		return nil
	}
	return q.HelmOptions.ValuesFileSchemes
}
