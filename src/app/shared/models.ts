import { models } from 'argo-ui';

export interface ApplicationList {
    /**
     * APIVersion defines the versioned schema of this representation of an object.
     * Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values.
     */
    apiVersion?: string;
    items: Application[];
    /**
     * Kind is a string value representing the REST resource this object represents.
     * Servers may infer this from the endpoint the client submits requests to.
     */
    kind?: string;
    metadata: models.ListMeta;
}

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

export type OperationPhase = 'InProgress' | 'Failed' | 'Succeeded';

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

export interface SyncOperationResult {
    resources: ResourceDetails[];
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
     * Overriden component parameters.
     */
    componentParameterOverrides: ComponentParameter[];
}

export interface ApplicationSpec {
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

export type ComparisonStatus = '' | 'Error' | 'Synced' | 'OutOfSync';

export const ComparisonStatuses = {
    Unknown: '',
    Error: 'Error' ,
    Synced: 'Synced' ,
    OutOfSync: 'OutOfSync',
};

export type HealthStatusCode = '' | 'Progressing' | 'Healthy' | 'Degraded';

export const HealthStatuses = {
    Unknown: '',
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
    error: string;
    namespace: string;
    server: string;
}

export interface ApplicationStatus {
    comparisonResult: ComparisonResult;
    recentDeployments: DeploymentInfo[];
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
