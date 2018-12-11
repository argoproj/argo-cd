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

export interface SyncOperationResource {group: string; kind: string; name: string; }

export interface SyncOperation {
    revision: string;
    prune: boolean;
    dryRun: boolean;
    resources?: SyncOperationResource[];
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

export type OperationPhase = 'Running' | 'Error' | 'Failed' | 'Succeeded' | 'Terminating';

export const OperationPhases = {
    Running: 'Running' as OperationPhase,
    Failed: 'Failed' as OperationPhase,
    Error: 'Error' as OperationPhase,
    Succeeded: 'Succeeded' as OperationPhase,
    Terminating: 'Terminating' as OperationPhase,
};

/**
 * OperationState contains information about state of currently performing operation on application.
 */
export interface OperationState {
    operation: Operation;
    phase: OperationPhase;
    message: string;
    syncResult: SyncOperationResult;
    startedAt: models.Time;
    finishedAt: models.Time;
}

export type HookType = 'PreSync' | 'Sync' | 'PostSync' | 'Skip';

export interface SyncOperationResult {
    resources: ResourceResult[];
}

export type ResultCode = 'Synced' | 'SyncFailed' | 'Pruned' | 'PruneSkipped';

export const ResultCodes = {
    Synced: 'Synced',
    SyncFailed: 'SyncFailed',
    Pruned: 'Pruned',
    PruneSkipped: 'PruneSkipped',
};

export interface ResourceResult {
    name: string;
    group: string;
    kind: string;
    version: string;
    namespace: string;
    status: ResultCode;
    message: string;
    hookType: HookType;
    hookPhase: OperationPhase;
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
     * Overridden component parameters.
     */
    componentParameterOverrides: ComponentParameter[];

    helm?: ApplicationSourceHelm;

    kustomize?: ApplicationSourceKustomize;

    ksonnet?: ApplicationSourceKsonnet;
}

export interface ApplicationSourceHelm {
    valueFiles: string[];
}

export interface ApplicationSourceKustomize {
    namePrefix: string;
}

export interface ApplicationSourceKsonnet {
    environment: string;
}

export interface SyncPolicy {
    automated?: { prune: boolean };
}

export interface ApplicationSpec {
    project: string;
    source: ApplicationSource;
    destination: ApplicationDestination;
    syncPolicy?: SyncPolicy;
}

/**
 * RevisionHistory contains information relevant to an application deployment
 */
export interface RevisionHistory {
    id: number;
    revision: string;
    componentParameterOverrides: ComponentParameter[];
    deployedAt: models.Time;
}

export type SyncStatusCode = 'Unknown' | 'Synced' | 'OutOfSync';

export const SyncStatuses = {
    Unknown: 'Unknown',
    Synced: 'Synced' ,
    OutOfSync: 'OutOfSync',
};

export type HealthStatusCode = 'Unknown' | 'Progressing' | 'Healthy' | 'Degraded' | 'Missing';

export const HealthStatuses = {
    Unknown: 'Unknown',
    Progressing: 'Progressing',
    Healthy: 'Healthy',
    Degraded: 'Degraded',
};

export interface HealthStatus {
    status: HealthStatusCode;
    message: string;
}

export type State = models.TypeMeta & { metadata: models.ObjectMeta } & { status: any, spec: any };

export interface ResourceStatus {
    group: string;
    version: string;
    kind: string;
    namespace: string;
    name: string;
    status: SyncStatusCode;
    health: HealthStatus;
    hook?: boolean;
}

export interface ResourceNode {
    kind: string;
    namespace: string;
    name: string;
    version: string;
    group: string;
    tags: string[];
    children: ResourceNode[];
    resourceVersion: string;
}

export interface ResourceDiff {
    group: string;
    kind: string;
    namespace: string;
    name: string;
    targetState: State;
    liveState: State;
    diff: string;
}

export interface SyncStatus {
    comparedTo: ApplicationSource;
    status: SyncStatusCode;
    revision: string;
}

export interface ApplicationCondition {
    type: string;
    message: string;
}

export interface ApplicationStatus {
    observedAt: models.Time;
    resources: ResourceStatus[];
    sync: SyncStatus;
    conditions?: ApplicationCondition[];
    history: RevisionHistory[];
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
    oidcConfig: {
        name: string;
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

export type AppSourceType = 'Helm' | 'Kustomize' | 'Ksonnet' | 'Directory';

export interface AppDetails {
    type: AppSourceType;
    path: string;
    ksonnet?: KsonnetAppSpec;
    helm?: HelmAppSpec;
    kustomize?: KustomizeAppSpec;
}

export interface AppInfo {
    type: string;
    path: string;
}

export interface HelmAppSpec {
    name: string;
    path: string;
    valueFiles: string[];
}

export interface KustomizeAppSpec {
    path: string;
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

export interface ProjectRole {
    description: string;
    policies: string[];
    name: string;
    jwtTokens: JwtToken[];
    groups: string[];
}

export interface JwtToken {
    iat: number;
    exp: number;
}

export interface GroupKind {
    group: string;
    kind: string;
}

export interface ProjectSpec {
    sourceRepos: string[];
    destinations: ApplicationDestination[];
    description: string;
    roles: ProjectRole[];
    clusterResourceWhitelist: GroupKind[];
    namespaceResourceBlacklist: GroupKind[];
}

export interface Project {
    apiVersion?: string;
    kind?: string;
    metadata: models.ObjectMeta;
    spec: ProjectSpec;
}

export type ProjectList = ItemsList<Project>;

export const DEFAULT_PROJECT_NAME = 'default';

export interface ManifestResponse {
    manifests: string[];
    namespace: string;
    server: string;
    revision: string;
    params: ComponentParameter[];
}
