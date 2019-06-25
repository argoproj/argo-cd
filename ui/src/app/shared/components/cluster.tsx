import {Tooltip} from 'argo-ui';
import * as React from 'react';
import {DataLoader} from '.';
import * as models from '../models';
import {services} from '../services';

export const clusterName = (name: string | null) => {
    return name || 'in-cluster';
};

export const clusterTitle = (cluster: models.Cluster) => {
    return `${clusterName(cluster.name)} (${cluster.server})`;
};

const clusterHTML = (cluster: models.Cluster, showUrl: boolean) => {
    const text = showUrl ? clusterTitle(cluster) : clusterName(cluster.name);
    return <Tooltip content={cluster.server}><span>{text}</span></Tooltip>;
};

interface Props {
    url: string;
    showUrl?: boolean;
}

export const Cluster = React.memo((props: Props) => {
    return (
        <DataLoader input={props.url}
                    load={(url) => services.clusters.get(url)}>{(cluster: models.Cluster) => clusterHTML(cluster, props.showUrl)}</DataLoader>
    );
});
