// +build !ignore_autogenerated

// This file was autogenerated by deepcopy-gen. Do not edit it manually!

package v1alpha1

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AppProject) DeepCopyInto(out *AppProject) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AppProject.
func (in *AppProject) DeepCopy() *AppProject {
	if in == nil {
		return nil
	}
	out := new(AppProject)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AppProject) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AppProjectList) DeepCopyInto(out *AppProjectList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]AppProject, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AppProjectList.
func (in *AppProjectList) DeepCopy() *AppProjectList {
	if in == nil {
		return nil
	}
	out := new(AppProjectList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AppProjectList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AppProjectSpec) DeepCopyInto(out *AppProjectSpec) {
	*out = *in
	if in.SourceRepos != nil {
		in, out := &in.SourceRepos, &out.SourceRepos
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Destinations != nil {
		in, out := &in.Destinations, &out.Destinations
		*out = make([]ApplicationDestination, len(*in))
		copy(*out, *in)
	}
	if in.Roles != nil {
		in, out := &in.Roles, &out.Roles
		*out = make([]ProjectRole, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AppProjectSpec.
func (in *AppProjectSpec) DeepCopy() *AppProjectSpec {
	if in == nil {
		return nil
	}
	out := new(AppProjectSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Application) DeepCopyInto(out *Application) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	if in.Operation != nil {
		in, out := &in.Operation, &out.Operation
		if *in == nil {
			*out = nil
		} else {
			*out = new(Operation)
			(*in).DeepCopyInto(*out)
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Application.
func (in *Application) DeepCopy() *Application {
	if in == nil {
		return nil
	}
	out := new(Application)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Application) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ApplicationCondition) DeepCopyInto(out *ApplicationCondition) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ApplicationCondition.
func (in *ApplicationCondition) DeepCopy() *ApplicationCondition {
	if in == nil {
		return nil
	}
	out := new(ApplicationCondition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ApplicationDestination) DeepCopyInto(out *ApplicationDestination) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ApplicationDestination.
func (in *ApplicationDestination) DeepCopy() *ApplicationDestination {
	if in == nil {
		return nil
	}
	out := new(ApplicationDestination)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ApplicationList) DeepCopyInto(out *ApplicationList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Application, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ApplicationList.
func (in *ApplicationList) DeepCopy() *ApplicationList {
	if in == nil {
		return nil
	}
	out := new(ApplicationList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ApplicationList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ApplicationSource) DeepCopyInto(out *ApplicationSource) {
	*out = *in
	if in.ComponentParameterOverrides != nil {
		in, out := &in.ComponentParameterOverrides, &out.ComponentParameterOverrides
		*out = make([]ComponentParameter, len(*in))
		copy(*out, *in)
	}
	if in.ValuesFiles != nil {
		in, out := &in.ValuesFiles, &out.ValuesFiles
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ApplicationSource.
func (in *ApplicationSource) DeepCopy() *ApplicationSource {
	if in == nil {
		return nil
	}
	out := new(ApplicationSource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ApplicationSpec) DeepCopyInto(out *ApplicationSpec) {
	*out = *in
	in.Source.DeepCopyInto(&out.Source)
	out.Destination = in.Destination
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ApplicationSpec.
func (in *ApplicationSpec) DeepCopy() *ApplicationSpec {
	if in == nil {
		return nil
	}
	out := new(ApplicationSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ApplicationStatus) DeepCopyInto(out *ApplicationStatus) {
	*out = *in
	in.ComparisonResult.DeepCopyInto(&out.ComparisonResult)
	if in.History != nil {
		in, out := &in.History, &out.History
		*out = make([]DeploymentInfo, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Parameters != nil {
		in, out := &in.Parameters, &out.Parameters
		*out = make([]ComponentParameter, len(*in))
		copy(*out, *in)
	}
	out.Health = in.Health
	if in.OperationState != nil {
		in, out := &in.OperationState, &out.OperationState
		if *in == nil {
			*out = nil
		} else {
			*out = new(OperationState)
			(*in).DeepCopyInto(*out)
		}
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]ApplicationCondition, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ApplicationStatus.
func (in *ApplicationStatus) DeepCopy() *ApplicationStatus {
	if in == nil {
		return nil
	}
	out := new(ApplicationStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ApplicationWatchEvent) DeepCopyInto(out *ApplicationWatchEvent) {
	*out = *in
	in.Application.DeepCopyInto(&out.Application)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ApplicationWatchEvent.
func (in *ApplicationWatchEvent) DeepCopy() *ApplicationWatchEvent {
	if in == nil {
		return nil
	}
	out := new(ApplicationWatchEvent)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Cluster) DeepCopyInto(out *Cluster) {
	*out = *in
	in.Config.DeepCopyInto(&out.Config)
	in.ConnectionState.DeepCopyInto(&out.ConnectionState)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Cluster.
func (in *Cluster) DeepCopy() *Cluster {
	if in == nil {
		return nil
	}
	out := new(Cluster)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterConfig) DeepCopyInto(out *ClusterConfig) {
	*out = *in
	in.TLSClientConfig.DeepCopyInto(&out.TLSClientConfig)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterConfig.
func (in *ClusterConfig) DeepCopy() *ClusterConfig {
	if in == nil {
		return nil
	}
	out := new(ClusterConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterList) DeepCopyInto(out *ClusterList) {
	*out = *in
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Cluster, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterList.
func (in *ClusterList) DeepCopy() *ClusterList {
	if in == nil {
		return nil
	}
	out := new(ClusterList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ComparisonResult) DeepCopyInto(out *ComparisonResult) {
	*out = *in
	in.ComparedAt.DeepCopyInto(&out.ComparedAt)
	in.ComparedTo.DeepCopyInto(&out.ComparedTo)
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = make([]ResourceState, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ComparisonResult.
func (in *ComparisonResult) DeepCopy() *ComparisonResult {
	if in == nil {
		return nil
	}
	out := new(ComparisonResult)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ComponentParameter) DeepCopyInto(out *ComponentParameter) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ComponentParameter.
func (in *ComponentParameter) DeepCopy() *ComponentParameter {
	if in == nil {
		return nil
	}
	out := new(ComponentParameter)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ConnectionState) DeepCopyInto(out *ConnectionState) {
	*out = *in
	if in.ModifiedAt != nil {
		in, out := &in.ModifiedAt, &out.ModifiedAt
		if *in == nil {
			*out = nil
		} else {
			*out = new(v1.Time)
			(*in).DeepCopyInto(*out)
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ConnectionState.
func (in *ConnectionState) DeepCopy() *ConnectionState {
	if in == nil {
		return nil
	}
	out := new(ConnectionState)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DeploymentInfo) DeepCopyInto(out *DeploymentInfo) {
	*out = *in
	if in.Params != nil {
		in, out := &in.Params, &out.Params
		*out = make([]ComponentParameter, len(*in))
		copy(*out, *in)
	}
	if in.ComponentParameterOverrides != nil {
		in, out := &in.ComponentParameterOverrides, &out.ComponentParameterOverrides
		*out = make([]ComponentParameter, len(*in))
		copy(*out, *in)
	}
	in.DeployedAt.DeepCopyInto(&out.DeployedAt)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DeploymentInfo.
func (in *DeploymentInfo) DeepCopy() *DeploymentInfo {
	if in == nil {
		return nil
	}
	out := new(DeploymentInfo)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HealthStatus) DeepCopyInto(out *HealthStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HealthStatus.
func (in *HealthStatus) DeepCopy() *HealthStatus {
	if in == nil {
		return nil
	}
	out := new(HealthStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HookStatus) DeepCopyInto(out *HookStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HookStatus.
func (in *HookStatus) DeepCopy() *HookStatus {
	if in == nil {
		return nil
	}
	out := new(HookStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *JwtToken) DeepCopyInto(out *JwtToken) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new JwtToken.
func (in *JwtToken) DeepCopy() *JwtToken {
	if in == nil {
		return nil
	}
	out := new(JwtToken)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Operation) DeepCopyInto(out *Operation) {
	*out = *in
	if in.Sync != nil {
		in, out := &in.Sync, &out.Sync
		if *in == nil {
			*out = nil
		} else {
			*out = new(SyncOperation)
			(*in).DeepCopyInto(*out)
		}
	}
	if in.Rollback != nil {
		in, out := &in.Rollback, &out.Rollback
		if *in == nil {
			*out = nil
		} else {
			*out = new(RollbackOperation)
			**out = **in
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Operation.
func (in *Operation) DeepCopy() *Operation {
	if in == nil {
		return nil
	}
	out := new(Operation)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OperationState) DeepCopyInto(out *OperationState) {
	*out = *in
	in.Operation.DeepCopyInto(&out.Operation)
	if in.SyncResult != nil {
		in, out := &in.SyncResult, &out.SyncResult
		if *in == nil {
			*out = nil
		} else {
			*out = new(SyncOperationResult)
			(*in).DeepCopyInto(*out)
		}
	}
	if in.RollbackResult != nil {
		in, out := &in.RollbackResult, &out.RollbackResult
		if *in == nil {
			*out = nil
		} else {
			*out = new(SyncOperationResult)
			(*in).DeepCopyInto(*out)
		}
	}
	in.StartedAt.DeepCopyInto(&out.StartedAt)
	if in.FinishedAt != nil {
		in, out := &in.FinishedAt, &out.FinishedAt
		if *in == nil {
			*out = nil
		} else {
			*out = new(v1.Time)
			(*in).DeepCopyInto(*out)
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OperationState.
func (in *OperationState) DeepCopy() *OperationState {
	if in == nil {
		return nil
	}
	out := new(OperationState)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProjectRole) DeepCopyInto(out *ProjectRole) {
	*out = *in
	if in.Policies != nil {
		in, out := &in.Policies, &out.Policies
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.JwtTokens != nil {
		in, out := &in.JwtTokens, &out.JwtTokens
		*out = make([]JwtToken, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProjectRole.
func (in *ProjectRole) DeepCopy() *ProjectRole {
	if in == nil {
		return nil
	}
	out := new(ProjectRole)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Repository) DeepCopyInto(out *Repository) {
	*out = *in
	in.ConnectionState.DeepCopyInto(&out.ConnectionState)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Repository.
func (in *Repository) DeepCopy() *Repository {
	if in == nil {
		return nil
	}
	out := new(Repository)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RepositoryList) DeepCopyInto(out *RepositoryList) {
	*out = *in
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Repository, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RepositoryList.
func (in *RepositoryList) DeepCopy() *RepositoryList {
	if in == nil {
		return nil
	}
	out := new(RepositoryList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceDetails) DeepCopyInto(out *ResourceDetails) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceDetails.
func (in *ResourceDetails) DeepCopy() *ResourceDetails {
	if in == nil {
		return nil
	}
	out := new(ResourceDetails)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceNode) DeepCopyInto(out *ResourceNode) {
	*out = *in
	if in.Children != nil {
		in, out := &in.Children, &out.Children
		*out = make([]ResourceNode, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceNode.
func (in *ResourceNode) DeepCopy() *ResourceNode {
	if in == nil {
		return nil
	}
	out := new(ResourceNode)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceState) DeepCopyInto(out *ResourceState) {
	*out = *in
	if in.ChildLiveResources != nil {
		in, out := &in.ChildLiveResources, &out.ChildLiveResources
		*out = make([]ResourceNode, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	out.Health = in.Health
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceState.
func (in *ResourceState) DeepCopy() *ResourceState {
	if in == nil {
		return nil
	}
	out := new(ResourceState)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RollbackOperation) DeepCopyInto(out *RollbackOperation) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RollbackOperation.
func (in *RollbackOperation) DeepCopy() *RollbackOperation {
	if in == nil {
		return nil
	}
	out := new(RollbackOperation)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SyncOperation) DeepCopyInto(out *SyncOperation) {
	*out = *in
	if in.SyncStrategy != nil {
		in, out := &in.SyncStrategy, &out.SyncStrategy
		if *in == nil {
			*out = nil
		} else {
			*out = new(SyncStrategy)
			(*in).DeepCopyInto(*out)
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SyncOperation.
func (in *SyncOperation) DeepCopy() *SyncOperation {
	if in == nil {
		return nil
	}
	out := new(SyncOperation)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SyncOperationResult) DeepCopyInto(out *SyncOperationResult) {
	*out = *in
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = make([]*ResourceDetails, len(*in))
		for i := range *in {
			if (*in)[i] == nil {
				(*out)[i] = nil
			} else {
				(*out)[i] = new(ResourceDetails)
				(*in)[i].DeepCopyInto((*out)[i])
			}
		}
	}
	if in.Hooks != nil {
		in, out := &in.Hooks, &out.Hooks
		*out = make([]*HookStatus, len(*in))
		for i := range *in {
			if (*in)[i] == nil {
				(*out)[i] = nil
			} else {
				(*out)[i] = new(HookStatus)
				(*in)[i].DeepCopyInto((*out)[i])
			}
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SyncOperationResult.
func (in *SyncOperationResult) DeepCopy() *SyncOperationResult {
	if in == nil {
		return nil
	}
	out := new(SyncOperationResult)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SyncStrategy) DeepCopyInto(out *SyncStrategy) {
	*out = *in
	if in.Apply != nil {
		in, out := &in.Apply, &out.Apply
		if *in == nil {
			*out = nil
		} else {
			*out = new(SyncStrategyApply)
			**out = **in
		}
	}
	if in.Hook != nil {
		in, out := &in.Hook, &out.Hook
		if *in == nil {
			*out = nil
		} else {
			*out = new(SyncStrategyHook)
			**out = **in
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SyncStrategy.
func (in *SyncStrategy) DeepCopy() *SyncStrategy {
	if in == nil {
		return nil
	}
	out := new(SyncStrategy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SyncStrategyApply) DeepCopyInto(out *SyncStrategyApply) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SyncStrategyApply.
func (in *SyncStrategyApply) DeepCopy() *SyncStrategyApply {
	if in == nil {
		return nil
	}
	out := new(SyncStrategyApply)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SyncStrategyHook) DeepCopyInto(out *SyncStrategyHook) {
	*out = *in
	out.SyncStrategyApply = in.SyncStrategyApply
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SyncStrategyHook.
func (in *SyncStrategyHook) DeepCopy() *SyncStrategyHook {
	if in == nil {
		return nil
	}
	out := new(SyncStrategyHook)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TLSClientConfig) DeepCopyInto(out *TLSClientConfig) {
	*out = *in
	if in.CertData != nil {
		in, out := &in.CertData, &out.CertData
		*out = make([]byte, len(*in))
		copy(*out, *in)
	}
	if in.KeyData != nil {
		in, out := &in.KeyData, &out.KeyData
		*out = make([]byte, len(*in))
		copy(*out, *in)
	}
	if in.CAData != nil {
		in, out := &in.CAData, &out.CAData
		*out = make([]byte, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TLSClientConfig.
func (in *TLSClientConfig) DeepCopy() *TLSClientConfig {
	if in == nil {
		return nil
	}
	out := new(TLSClientConfig)
	in.DeepCopyInto(out)
	return out
}
