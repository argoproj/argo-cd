import { models } from 'argo-ui';

interface ItemsList<T> {
    /**
     * APIVersion defines the versioned schema of this representation of an object.
     * Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values.
     */
    apiVersion?: string;
    items: T[];
    /**
     * Kind is a string value representing the REST resource this object represents.
     * Servers may infer this from the endpoint the client submits requests to.
     */
    kind?: string;
    metadata: models.ListMeta;
}

export interface ApplicationList extends ItemsList<Application> {}

export interface SyncOperation {
    revision: string;
    prune: boolean;
    dryRun: boolean;
}

export interface RollbackOperation {
    id: number;
    prune: boolean;
    dryRun: boolean;
}

export interface Operation {
    sync: SyncOperation;
    rollback: RollbackOperation;
}

export type OperationPhase = 'InProgress' | 'Failed' | 'Succeeded' | 'Terminating';

/**
 * OperationState contains information about state of currently performing operation on application.
 */
export interface OperationState {
    operation: Operation;
    phase: OperationPhase;
    message: string;
    syncResult: SyncOperationResult;
    rollbackResult: SyncOperationResult;
    startedAt: models.Time;
    finishedAt: models.Time;
}

export type HookType = 'PreSync' | 'Sync' | 'PostSync' | 'Skip';

export interface HookStatus {
    /**
     * Name is the resource name
     */
    name: string;
    /**
     * Name is the resource name
     */
    kind: string;
    /**
     * Name is the resource name
     */
    apiVersion: string;
    /**
     * Type is the type of hook (e.g. PreSync, Sync, PostSync, Skip)
     */
    type: HookType;
    /**
     * Status a simple, high-level summary of where the resource is in its lifecycle
     */
    status: OperationPhase;
    /**
     * A human readable message indicating details about why the resource is in this condition.
     */
    message: string;
}

export interface SyncOperationResult {
    resources: ResourceDetails[];
    hooks?: HookStatus[];
}

export interface ResourceDetails {
    name: string;
    kind: string;
    namespace: string;
    message: string;
}

export interface Application {
    apiVersion?: string;
    kind?: string;
    metadata: models.ObjectMeta;
    spec: ApplicationSpec;
    status: ApplicationStatus;
    operation?: Operation;
}

type WatchType = 'ADDED' | 'MODIFIED' | 'DELETED' | 'ERROR';

export interface ApplicationWatchEvent {
    type: WatchType;
    application: Application;
}

export interface ComponentParameter {
    component: string;
    name: string;
    value: string;
}

export interface ApplicationDestination {
    /**
     * Server overrides the environment server value in the ksonnet app.yaml
     */
    server: string;
    /**
     * Namespace overrides the environment namespace value in the ksonnet app.yaml
     */
    namespace: string;
}

export interface ApplicationSource {
    targetRevision: string;
    /**
     * RepoURL is repository URL which contains application project.
     */
    repoURL: string;

    /**
     * Path is a directory path within repository which contains ksonnet project.
     */
    path: string;

    /**
     * Environment is a ksonnet project environment name.
     */
    environment: string;

    /**
     * Overridden component parameters.
     */
    componentParameterOverrides: ComponentParameter[];
}

export interface ApplicationSpec {
    project: string;
    source: ApplicationSource;
    destination: ApplicationDestination;
}

/**
 * DeploymentInfo contains information relevant to an application deployment
 */
export interface DeploymentInfo {
    id: number;
    revision: string;
    params: ComponentParameter[];
    componentParameterOverrides: ComponentParameter[];
    deployedAt: models.Time;
}

export type ComparisonStatus = 'Unknown' | 'Synced' | 'OutOfSync';

export const ComparisonStatuses = {
    Unknown: 'Unknown',
    Synced: 'Synced' ,
    OutOfSync: 'OutOfSync',
};

export type HealthStatusCode = 'Unknown' | 'Progressing' | 'Healthy' | 'Degraded';

export const HealthStatuses = {
    Unknown: 'Unknown',
    Progressing: 'Progressing',
    Healthy: 'Healthy',
    Degraded: 'Degraded',
};

export interface HealthStatus {
    status: HealthStatusCode;
    statusDetails: string;
}

export type State = models.TypeMeta & { metadata: models.ObjectMeta } & { status: any, spec: any };

export interface ResourceNode {
    state: State;
    children: ResourceNode[];
}

export interface ResourceState {
    targetState: State;
    liveState: State;
    status: ComparisonStatus;
    health: HealthStatus;
    childLiveResources: ResourceNode[];
}

export interface ComparisonResult {
    comparedAt: models.Time;
    comparedTo: ApplicationSource;
    status: ComparisonStatus;
    resources: ResourceState[];
    namespace: string;
    server: string;
}

export interface ApplicationCondition {
    type: string;
    message: string;
}

export interface ApplicationStatus {
    comparisonResult: ComparisonResult;
    conditions?: ApplicationCondition[];
    history: DeploymentInfo[];
    parameters: ComponentParameter[];
    health: HealthStatus;
    operationState?: OperationState;
}

export interface LogEntry {
    content: string;
    timeStamp: models.Time;
}

export interface AuthSettings {
    url: string;
    dexConfig: {
        connectors: {
            name: string;
            type: string;
        }[];
    };
}

export type ConnectionStatus = 'Unknown' | 'Successful' | 'Failed';

export const ConnectionStatuses = {
    Unknown: 'Unknown' ,
    Failed: 'Failed' ,
    Successful: 'Successful',
};

export interface ConnectionState {
    status: ConnectionStatus;
    message: string;
    attemptedAt: models.Time;
}

export interface Repository {
    repo: string;
    connectionState: ConnectionState;
}

export interface RepositoryList extends ItemsList<Repository> {}

export interface Cluster {
    name: string;
    server: string;
    connectionState: ConnectionState;
}

export interface ClusterList extends ItemsList<Cluster> {}

export interface KsonnetEnvironment {
    k8sVersion: string;
    path: string;
    destination: { server: string; namespace: string; };
}

export interface KsonnetAppSpec {
    name: string;
    path: string;
    environments: { [key: string]: KsonnetEnvironment; };
}

export interface ObjectReference {
    kind: string;
    namespace: string;
    name: string;
    uid: string;
    apiVersion: string;
    resourceVersion: string;
    fieldPath: string;
}

export interface EventSource {
    component: string;
    host: string;
}

export interface EventSeries {
    count: number;
    lastObservedTime: models.Time;
    state: string;
}

export interface Event {
    apiVersion?: string;
    kind?: string;
    metadata: models.ObjectMeta;
    involvedObject: ObjectReference;
    reason: string;
    message: string;
    source: EventSource;
    firstTimestamp: models.Time;
    lastTimestamp: models.Time;
    count: number;
    type: string;
    eventTime: models.Time;
    series: EventSeries;
    action: string;
    related: ObjectReference;
    reportingController: string;
    reportingInstance: string;
}

export interface EventList extends ItemsList<Event> {}

export interface ProjectSpec {
    destinations: ApplicationDestination[];
}

export interface Project {
    apiVersion?: string;
    kind?: string;
    metadata: models.ObjectMeta;
    spec: ProjectSpec;
}

export type ProjectList = ItemsList<Project>;

export const DEFAULT_PROJECT_NAME = 'default';
