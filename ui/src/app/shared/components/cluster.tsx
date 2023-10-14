import {Tooltip} from 'argo-ui';
import * as React from 'react';
import {DataLoader} from '.';
import * as models from '../models';
import {services} from '../services';

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

async function getCluster(clusters: Promise<models.Cluster[]>, server: string, name: string): Promise<models.Cluster> {
    let cluster: models.Cluster;
    if (clusters) {
        cluster = await clusters.then(items => items.find(item => item.server === server || item.name === name));
    } else {
        try {
            cluster = await services.clusters.get(server, name);
        } catch {
            cluster = null;
        }
    }
    if (!cluster) {
        cluster = {
            name: server,
            server
        };
    }
    return cluster;
}

export const ClusterCtx = React.createContext<Promise<Array<models.Cluster>>>(null);

export const Cluster = (props: {server: string; name?: string; showUrl?: boolean}) => (
    <ClusterCtx.Consumer>
        {clusters => (
            <DataLoader input={props} loadingRenderer={() => <span>{props.server}</span>} load={input => getCluster(clusters, input.server, input.name)}>
                {(cluster: models.Cluster) => clusterHTML(cluster, props.showUrl)}
            </DataLoader>
        )}
    </ClusterCtx.Consumer>
);
