package v1alpha1

import (
	"maps"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/health"
)

// DeepCopyInto copies the receiver into out.
func (in *ApplicationListStats) DeepCopyInto(out *ApplicationListStats) {
	*out = *in
	if in.TotalBySyncStatus != nil {
		in, out := &in.TotalBySyncStatus, &out.TotalBySyncStatus
		*out = make(map[SyncStatusCode]int64, len(*in))
		maps.Copy((*out), *in)
	}
	if in.TotalByHealthStatus != nil {
		in, out := &in.TotalByHealthStatus, &out.TotalByHealthStatus
		*out = make(map[health.HealthStatusCode]int64, len(*in))
		maps.Copy((*out), *in)
	}
	if in.AutoSyncCount != nil {
		in, out := &in.AutoSyncCount, &out.AutoSyncCount
		*out = make(map[string]int64, len(*in))
		maps.Copy((*out), *in)
	}
	if in.Destinations != nil {
		in, out := &in.Destinations, &out.Destinations
		*out = make([]ApplicationDestination, len(*in))
		copy(*out, *in)
	}
	if in.Namespaces != nil {
		in, out := &in.Namespaces, &out.Namespaces
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make([]ApplicationLabelStats, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Repos != nil {
		in, out := &in.Repos, &out.Repos
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy returns a deep copy of ApplicationListStats.
func (in *ApplicationListStats) DeepCopy() *ApplicationListStats {
	if in == nil {
		return nil
	}
	out := new(ApplicationListStats)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies the receiver into out.
func (in *ApplicationLabelStats) DeepCopyInto(out *ApplicationLabelStats) {
	*out = *in
	if in.Values != nil {
		in, out := &in.Values, &out.Values
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy returns a deep copy of ApplicationLabelStats.
func (in *ApplicationLabelStats) DeepCopy() *ApplicationLabelStats {
	if in == nil {
		return nil
	}
	out := new(ApplicationLabelStats)
	in.DeepCopyInto(out)
	return out
}
