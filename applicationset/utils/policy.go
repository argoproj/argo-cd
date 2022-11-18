package utils

// Policy allows to apply different rules to a set of changes.
type Policy interface {
	Update() bool
	Delete() bool
}

// Policies is a registry of available policies.
var Policies = map[string]Policy{
	"sync":          &SyncPolicy{},
	"create-only":   &CreateOnlyPolicy{},
	"create-update": &CreateUpdatePolicy{},
}

type SyncPolicy struct{}

func (p *SyncPolicy) Update() bool {
	return true
}

func (p *SyncPolicy) Delete() bool {
	return true
}

type CreateUpdatePolicy struct{}

func (p *CreateUpdatePolicy) Update() bool {
	return true
}

func (p *CreateUpdatePolicy) Delete() bool {
	return false
}

type CreateOnlyPolicy struct{}

func (p *CreateOnlyPolicy) Update() bool {
	return false
}

func (p *CreateOnlyPolicy) Delete() bool {
	return false
}
