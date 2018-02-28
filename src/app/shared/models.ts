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

export interface Application {
    apiVersion?: string;
    kind?: string;
    metadata: models.ObjectMeta;
    spec: ApplicationSpec;
    status: ApplicationStatus;
}

type WatchType = 'ADDED' | 'MODIFIED' | 'DELETED' | 'ERROR';

export interface ApplicationWatchEvent {
    type: WatchType;
    application: Application;
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
}

export interface ApplicationSpec {
    source: ApplicationSource;
}

export type ComparisonStatus = '' | 'Error' | 'Equal' | 'Different';

export const ComparisonStatuses = {
    Unknown: '',
    Error: 'Error' ,
    Equal: 'Equal' ,
    Different: 'Different',
};

export interface ComparisonResult {
    comparedAt: models.Time;
    comparedTo: ApplicationSource;
    status: ComparisonStatus;
    targetState: string[];
    deltaDiffs: string[];
    error: string;
}

export interface ApplicationStatus {
    comparisonResult: ComparisonResult;
}
