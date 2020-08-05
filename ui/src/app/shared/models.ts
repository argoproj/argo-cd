import {models} from 'argo-ui';

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

export interface SyncOperationResource {
    group: string;
    kind: string;
    name: string;
}

export interface SyncStrategy {
    apply: {force?: boolean} | null;
    hook: {force?: boolean} | null;
}

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

export interface OperationInitiator {
    username: string;
    automated: boolean;
}

export interface Operation {
    sync: SyncOperation;
    initiatedBy: OperationInitiator;
}

export type OperationPhase = 'Running' | 'Error' | 'Failed' | 'Succeeded' | 'Terminating';

export const OperationPhases = {
    Running: 'Running' as OperationPhase,
    Failed: 'Failed' as OperationPhase,
    Error: 'Error' as OperationPhase,
    Succeeded: 'Succeeded' as OperationPhase,
    Terminating: 'Terminating' as OperationPhase
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

export type HookType = 'PreSync' | 'Sync' | 'PostSync' | 'SyncFail' | 'Skip';

export interface RevisionMetadata {
    author?: string;
    date: models.Time;
    tags?: string[];
    message?: string;
    signatureInfo?: string;
}

export interface SyncOperationResult {
    resources: ResourceResult[];
    revision: string;
}

export type ResultCode = 'Synced' | 'SyncFailed' | 'Pruned' | 'PruneSkipped';

export const ResultCodes = {
    Synced: 'Synced',
    SyncFailed: 'SyncFailed',
    Pruned: 'Pruned',
    PruneSkipped: 'PruneSkipped'
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

export const AnnotationRefreshKey = 'argocd.argoproj.io/refresh';

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

export interface OrphanedResource {
    group: string;
    kind: string;
    name: string;
}

export interface ApplicationSource {
    targetRevision: string;
    /**
     * RepoURL is repository URL which contains application project.
     */
    repoURL: string;

    /**
     * Path is a directory path within repository which
     */
    path?: string;

    chart?: string;

    helm?: ApplicationSourceHelm;

    kustomize?: ApplicationSourceKustomize;

    ksonnet?: ApplicationSourceKsonnet;

    plugin?: ApplicationSourcePlugin;

    directory?: ApplicationSourceDirectory;
}

export interface ApplicationSourceHelm {
    valueFiles: string[];
    values?: string;
    parameters: HelmParameter[];
    fileParameters: HelmFileParameter[];
}

export interface ApplicationSourceKustomize {
    namePrefix: string;
    nameSuffix: string;
    images: string[];
    version: string;
}

export interface ApplicationSourceKsonnet {
    environment: string;
    parameters: KsonnetParameter[];
}

export interface EnvEntry {
    name: string;
    value: string;
}

export interface ApplicationSourcePlugin {
    name: string;
    env: EnvEntry[];
}

export interface JsonnetVar {
    name: string;
    value: string;
    code: boolean;
}

interface ApplicationSourceJsonnet {
    extVars: JsonnetVar[];
    tlas: JsonnetVar[];
}

export interface ApplicationSourceDirectory {
    recurse: boolean;
    jsonnet?: ApplicationSourceJsonnet;
}

export interface Automated {
    prune: boolean;
    selfHeal: boolean;
}

export interface SyncPolicy {
    automated?: Automated;
    syncOptions?: string[];
}

export interface Info {
    name: string;
    value: string;
}

export interface ApplicationSpec {
    project: string;
    source: ApplicationSource;
    destination: ApplicationDestination;
    syncPolicy?: SyncPolicy;
    info?: Info[];
    revisionHistoryLimit?: number;
}

/**
 * RevisionHistory contains information relevant to an application deployment
 */
export interface RevisionHistory {
    id: number;
    revision: string;
    source: ApplicationSource;
    deployStartedAt: models.Time;
    deployedAt: models.Time;
}

export type SyncStatusCode = 'Unknown' | 'Synced' | 'OutOfSync';

export const SyncStatuses: {[key: string]: SyncStatusCode} = {
    Unknown: 'Unknown',
    Synced: 'Synced',
    OutOfSync: 'OutOfSync'
};

export type HealthStatusCode = 'Unknown' | 'Progressing' | 'Healthy' | 'Suspended' | 'Degraded' | 'Missing';

export const HealthStatuses: {[key: string]: HealthStatusCode} = {
    Unknown: 'Unknown',
    Progressing: 'Progressing',
    Suspended: 'Suspended',
    Healthy: 'Healthy',
    Degraded: 'Degraded',
    Missing: 'Missing'
};

export interface HealthStatus {
    status: HealthStatusCode;
    message: string;
}

export type State = models.TypeMeta & {metadata: models.ObjectMeta} & {status: any; spec: any};

export interface ResourceStatus {
    group: string;
    version: string;
    kind: string;
    namespace: string;
    name: string;
    status: SyncStatusCode;
    health: HealthStatus;
    hook?: boolean;
    requiresPruning?: boolean;
}

export interface ResourceRef {
    uid: string;
    kind: string;
    namespace: string;
    name: string;
    version: string;
    group: string;
}

export interface ResourceNetworkingInfo {
    targetLabels: {[name: string]: string};
    targetRefs: ResourceRef[];
    labels: {[name: string]: string};
    ingress: LoadBalancerIngress[];
    externalURLs: string[];
}

export interface LoadBalancerIngress {
    hostname: string;
    ip: string;
}

export interface ResourceNode extends ResourceRef {
    parentRefs: ResourceRef[];
    info: {name: string; value: string}[];
    networkingInfo?: ResourceNetworkingInfo;
    images?: string[];
    resourceVersion: string;
    createdAt?: models.Time;
}

export interface ApplicationTree {
    nodes: ResourceNode[];
    orphanedNodes: ResourceNode[];
}

export interface ResourceID {
    group: string;
    kind: string;
    namespace: string;
    name: string;
}

export interface ResourceDiff extends ResourceID {
    targetState: State;
    liveState: State;
    predictedLiveState: State;
    normalizedLiveState: State;
    hook: boolean;
}

export interface SyncStatus {
    comparedTo: ApplicationSource;
    status: SyncStatusCode;
    revision: string;
}

export interface ApplicationCondition {
    type: string;
    message: string;
    lastTransitionTime: string;
}

export interface ApplicationSummary {
    externalURLs?: string[];
    images?: string[];
}

export interface ApplicationStatus {
    observedAt: models.Time;
    resources: ResourceStatus[];
    sync: SyncStatus;
    conditions?: ApplicationCondition[];
    history: RevisionHistory[];
    health: HealthStatus;
    operationState?: OperationState;
    summary?: ApplicationSummary;
}

export interface JwtTokens {
    items: JwtToken[];
}
export interface AppProjectStatus {
    jwtTokensByRole: {[name: string]: JwtTokens};
}

export interface LogEntry {
    content: string;
    timeStamp: models.Time;
}

// describes plugin settings
export interface Plugin {
    name: string;
}

export interface AuthSettings {
    url: string;
    statusBadgeEnabled: boolean;
    googleAnalytics: {
        trackingID: string;
        anonymizeUsers: boolean;
    };
    dexConfig: {
        connectors: {
            name: string;
            type: string;
        }[];
    };
    oidcConfig: {
        name: string;
    };
    help: {
        chatUrl: string;
        chatText: string;
    };
    plugins: Plugin[];
    userLoginsDisabled: boolean;
    kustomizeVersions: string[];
}

export interface UserInfo {
    loggedIn: boolean;
    username: string;
    iss: string;
    groups: string[];
}

export type ConnectionStatus = 'Unknown' | 'Successful' | 'Failed';

export const ConnectionStatuses = {
    Unknown: 'Unknown',
    Failed: 'Failed',
    Successful: 'Successful'
};

export interface ConnectionState {
    status: ConnectionStatus;
    message: string;
    attemptedAt: models.Time;
}

export interface RepoCert {
    serverName: string;
    certType: string;
    certSubType: string;
    certData: string;
    certInfo: string;
}

export interface RepoCertList extends ItemsList<RepoCert> {}

export interface Repository {
    repo: string;
    type?: string;
    name?: string;
    connectionState: ConnectionState;
}

export interface RepositoryList extends ItemsList<Repository> {}

export interface RepoCreds {
    url: string;
    username?: string;
}

export interface RepoCredsList extends ItemsList<RepoCreds> {}

export interface Cluster {
    name: string;
    server: string;
    namespaces?: [];
    refreshRequestedAt?: models.Time;
    config?: {
        awsAuthConfig?: {
            clusterName: string;
        };
    };
    info?: {
        applicationsCount: number;
        serverVersion: string;
        connectionState: ConnectionState;
        cacheInfo: ClusterCacheInfo;
    };
}

export interface ClusterCacheInfo {
    resourcesCount: number;
    apisCount: number;
    lastCacheSyncTime: models.Time;
}

export interface ClusterList extends ItemsList<Cluster> {}

export interface KsonnetEnvironment {
    k8sVersion: string;
    path: string;
    destination: {server: string; namespace: string};
}

export interface KsonnetParameter {
    component: string;
    name: string;
    value: string;
}

export interface KsonnetAppSpec {
    name: string;
    path: string;
    environments: {[key: string]: KsonnetEnvironment};
    parameters: KsonnetParameter[];
}

export interface HelmChart {
    name: string;
    versions: string[];
}

export type AppSourceType = 'Helm' | 'Kustomize' | 'Ksonnet' | 'Directory' | 'Plugin';

export interface RepoAppDetails {
    type: AppSourceType;
    path: string;
    ksonnet?: KsonnetAppSpec;
    helm?: HelmAppSpec;
    kustomize?: KustomizeAppSpec;
    plugin?: PluginAppSpec;
    directory?: {};
}

export interface AppInfo {
    type: string;
    path: string;
}

export interface HelmParameter {
    name: string;
    value: string;
}

export interface HelmFileParameter {
    name: string;
    path: string;
}

export interface HelmAppSpec {
    name: string;
    path: string;
    valueFiles: string[];
    values?: string;
    parameters: HelmParameter[];
    fileParameters: HelmFileParameter[];
}

export interface KustomizeAppSpec {
    path: string;
    images?: string[];
}

export interface PluginAppSpec {
    name: string;
    env: EnvEntry[];
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
    groups: string[];
}

export interface JwtToken {
    iat: number;
    exp: number;
    id: string;
}

export interface GroupKind {
    group: string;
    kind: string;
}

export interface ProjectSignatureKey {
    keyID: string;
}

export interface ProjectSpec {
    sourceRepos: string[];
    destinations: ApplicationDestination[];
    description: string;
    roles: ProjectRole[];
    clusterResourceWhitelist: GroupKind[];
    namespaceResourceBlacklist: GroupKind[];
    namespaceResourceWhitelist: GroupKind[];
    signatureKeys: ProjectSignatureKey[];
    orphanedResources?: {warn?: boolean; ignore: OrphanedResource[]};
    syncWindows?: SyncWindows;
}

export type SyncWindows = SyncWindow[];

export interface SyncWindow {
    kind: string;
    schedule: string;
    duration: string;
    applications: string[];
    namespaces: string[];
    clusters: string[];
    manualSync: boolean;
}

export interface Project {
    apiVersion?: string;
    kind?: string;
    metadata: models.ObjectMeta;
    spec: ProjectSpec;
    status: AppProjectStatus;
}

export type ProjectList = ItemsList<Project>;

export const DEFAULT_PROJECT_NAME = 'default';

export interface ManifestResponse {
    manifests: string[];
    namespace: string;
    server: string;
    revision: string;
}

export interface ResourceActionParam {
    name: string;
    value: string;
    type: string;
    default: string;
}

export interface ResourceAction {
    name: string;
    params: ResourceActionParam[];
    disabled: boolean;
}

export interface SyncWindowsState {
    windows: SyncWindow[];
}

export interface ApplicationSyncWindowState {
    activeWindows: SyncWindow[];
    assignedWindows: SyncWindow[];
    canSync: boolean;
}

export interface VersionMessage {
    Version: string;
}

export interface Token {
    id: string;
    issuedAt: number;
    expiresAt: number;
}

export interface Account {
    name: string;
    enabled: boolean;
    capabilities: string[];
    tokens: Token[];
}

export interface GnuPGPublicKey {
    keyID?: string;
    fingerprint?: string;
    subType?: string;
    owner?: string;
    keyData?: string;
}

export interface GnuPGPublicKeyList extends ItemsList<GnuPGPublicKey> {}
