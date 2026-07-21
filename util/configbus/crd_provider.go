package configbus

// CRDProvider resolves config only from the ArgoCDConfiguration CRD. Until the
// CRD is introduced, every getter returns ErrNotConfigured so ChainProvider
// falls through to later links.
type CRDProvider struct {
	notConfiguredProvider
	// source is reserved until the CRD is introduced. Nil means no CRD is available.
	source any
}

// NewCRDProvider constructs a CRDProvider. Pass nil until the CRD source exists.
func NewCRDProvider(source any) *CRDProvider {
	return &CRDProvider{source: source}
}

// Ensure CRDProvider implements Provider.
var _ Provider = (*CRDProvider)(nil)
