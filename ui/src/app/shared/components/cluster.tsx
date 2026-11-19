import {Tooltip} from 'argo-ui';
import * as React from 'react';
import {useQuery} from '@tanstack/react-query';
import * as models from '../models';
import {clusterQueries} from '../queries';
import {loadFailed} from '../query-client';

export const clusterName = (name: string | null) => {
    return name || 'Unknown';
};

export const clusterTitle = (cluster: models.Cluster) => {
    return `${clusterName(cluster.name)} (${cluster.server})`;
};

const clusterHTML = (cluster: models.Cluster, showUrl: boolean) => {
    const text = showUrl ? clusterTitle(cluster) : clusterName(cluster.name);
    return (
        <Tooltip content={cluster.server}>
            <span>{text}</span>
        </Tooltip>
    );
};

export const ClusterCtx = React.createContext<Promise<Array<models.Cluster>>>(null);

export const Cluster = (props: {server: string; name?: string; showUrl?: boolean}) => {
    // List pages provide ClusterCtx and load all clusters once; elsewhere each cluster is looked up on its own.
    const inList = React.useContext(ClusterCtx) !== null;
    // The list page refreshes the list when it mounts, so its rows only read it, and rows mounting
    // after a failure don't refetch it ("try again" still does).
    const list = useQuery({...clusterQueries.list(), enabled: inList, staleTime: Infinity, retryOnMount: false});
    const lookup = useQuery({...clusterQueries.lookup(props.server, props.name), enabled: !inList});

    if (inList && list.isError && !list.data) {
        return loadFailed(list.refetch);
    }
    const loaded = inList ? list.data !== undefined : lookup.data !== undefined;
    if (!loaded) {
        return <span>{props.server}</span>;
    }
    // Match by name only when one is given, so a cluster without a name doesn't match every nameless lookup.
    const cluster = (inList ? list.data.find(item => item.server === props.server || (props.name && item.name === props.name)) : lookup.data) || {
        name: props.server,
        server: props.server
    };
    return clusterHTML(cluster, props.showUrl);
};
