import * as React from 'react';
import {ArgoAutocomplete} from '../../../settings/components/project/card/autocomplete';

// https://kubernetes.io/docs/reference/kubectl/overview/#resource-types

export enum ResourceKinds {
    ANY = '*',
    Binding = 'bindings',
    ComponentStatus = 'componentstatuses',
    ConfigMap = 'configmaps',
    Endpoints = 'endpoints',
    LimitRange = 'limitranges',
    Namespace = 'namespaces',
    Node = 'nodes',
    PersistentVolumeClaim = 'persistentvolumeclaims',
    PersistentVolume = 'persistentvolumes',
    Pod = 'pods',
    PodTemplate = 'podtemplates',
    ReplicationController = 'replicationcontrollers',
    ResourceQuota = 'resourcequotas',
    Secret = 'secrets',
    ServiceAccount = 'serviceaccounts',
    Service = 'services',
    MutatingWebhookConfiguration = 'mutatingwebhookconfigurations',
    ValidatingWebhookConfiguration = 'validatingwebhookconfigurations',
    CustomResourceDefinition = 'customresourcedefinitions',
    APIService = 'apiservices',
    ControllerRevision = 'controllerrevisions',
    DaemonSet = 'daemonsets',
    Deployment = 'deployments',
    ReplicaSet = 'replicasets',
    StatefulSet = 'statefulsets',
    TokenReview = 'tokenreviews',
    LocalSubjectAccessReview = 'localsubjectaccessreviews',
    SelfSubjectAccessReview = 'selfsubjectaccessreviews',
    SelfSubjectRulesReview = 'selfsubjectrulesreviews',
    SubjectAccessReview = 'subjectaccessreviews',
    HorizontalPodAutoscaler = 'horizontalpodautoscalers',
    CronJob = 'cronjobs',
    Job = 'jobs',
    CertificateSigningRequest = 'certificatesigningrequests',
    Lease = 'leases',
    Event = 'events',
    Ingress = 'ingresses',
    NetworkPolicy = 'networkpolicies',
    PodDisruptionBudget = 'poddisruptionbudgets',
    ClusterRoleBinding = 'clusterrolebindings',
    ClusterRole = 'clusterroles',
    RoleBinding = 'rolebindings',
    Role = 'roles',
    PriorityClass = 'priorityclasses',
    CSIDriver = 'csidrivers',
    CSINode = 'csinodes',
    StorageClass = 'storageclasses',
    VolumeAttachment = 'volumeattachments'
}

export type ResourceKind = keyof ResourceKinds;

interface ResourceKindProps {
    onChange: (value: ResourceKinds) => void;
    init: ResourceKind;
    placeholder: string;
}

export class ResourceKindSelector extends React.Component<ResourceKindProps> {
    public render() {
        return <ArgoAutocomplete onChange={this.props.onChange} init={this.props.init} values={Object.keys(ResourceKinds)} placeholder={this.props.placeholder} />;
    }
}
