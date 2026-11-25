import {Tooltip} from 'argo-ui';
import * as React from 'react';
import {noop, useQuery, useQueryClient} from '@tanstack/react-query';
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

// True on list pages, which load all clusters once; elsewhere each cluster is looked up on its own.
export const ClusterCtx = React.createContext(false);

// Refreshes the cluster list once per view of a list page. A layout effect starts the request before a page
// opened by a click is painted, so its rows don't briefly show a failure cached by a previous visit.
export const useRefreshClusterList = () => {
    const queryClient = useQueryClient();
    React.useLayoutEffect(() => {
        // A failure is shown by the components reading the list, so it is not handled here.
        queryClient.query(clusterQueries.list()).catch(noop);
    }, [queryClient]);
};

// Reads the cluster list on a list page. The page refreshes it when it mounts, so components only read
// it, and components mounting after a failure don't refetch it ("try again" still does).
export const useClusterList = (enabled = true) => useQuery({...clusterQueries.list(), enabled, staleTime: Infinity, retryOnMount: false});

export const Cluster = (props: {server: string; name?: string; showUrl?: boolean}) => {
    const inList = React.useContext(ClusterCtx);
    const list = useClusterList(inList);
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
