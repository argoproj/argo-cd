import * as React from 'react';
import {Application} from '../../../shared/models';
import {ApplicationURLs} from '../application-urls';
import {ComparisonStatusIcon, HealthStatusIcon, refreshLinkAttrs} from '../utils';
import {ClusterCtx, clusterTitle, GetCluster} from '../../../shared/components';
import {Context} from '../../../shared/context';
import {ActionButton, Checkbox, Flexy, InfoItemRow, useData} from 'argo-ux';
import {faSync, faRedo, faTimesCircle} from '@fortawesome/free-solid-svg-icons';

import './application-tile.scss';

type AppAction = (app: string) => void;

export const ApplicationTile = (props: {
    app: Application;
    selected?: boolean;
    ref: React.MutableRefObject<any>;
    onSelect?: (selected: boolean) => void;
    syncApplication: AppAction;
    refreshApplication: AppAction;
    deleteApplication: AppAction;
}) => {
    const {app, selected, ref, syncApplication, refreshApplication, deleteApplication} = props;
    const appContext = React.useContext(Context);
    const clusterContext = React.useContext(ClusterCtx);
    const [cluster, loading] = useData(() => GetCluster(clusterContext, app.spec.destination.server, app.spec.destination.name));

    return (
        <div key={app.metadata.name} className='column column-block'>
            <div
                ref={ref}
                className={`application-tile argo-table-list__row applications-list__entry applications-list__entry--comparison-${
                    app.status.sync.status
                } applications-list__entry--health-${app.status.health.status} ${selected ? 'applications-tiles__selected' : ''}`}>
                <div onClick={e => appContext.navigation.goto(`/applications/${app.metadata.name}`, {}, {event: e})}>
                    <Flexy className='application-tile__header'>
                        <Checkbox
                            onChange={val => {
                                if (props.onSelect) {
                                    props.onSelect(val);
                                }
                            }}
                        />
                        {app.metadata.name}
                        <div style={{marginLeft: 'auto'}}>
                            <HealthStatusIcon state={app.status.health} />
                            &nbsp;
                            <ComparisonStatusIcon status={app.status.sync.status} />
                        </div>
                    </Flexy>
                    <ApplicationURLs urls={app.status.summary.externalURLs} />
                    <InfoItemRow label='Namespace' items={[{content: app.spec.destination.namespace}]} lightweight={true} />
                    <InfoItemRow label='Project' items={[{content: app.spec.project}]} lightweight />
                    <InfoItemRow label='Repo' items={[{content: app.spec.source.repoURL, truncate: true}]} lightweight />
                    <InfoItemRow label='Destination' items={[{content: loading ? 'Loading...' : clusterTitle(cluster), truncate: true}]} lightweight />
                    {app.metadata.labels && (
                        <InfoItemRow
                            label='Labels'
                            items={Object.keys(app.metadata.labels).map(l => {
                                return {content: l};
                            })}
                        />
                    )}
                    <InfoItemRow label='Path' items={[{content: app.spec.source.path}]} lightweight />
                    <InfoItemRow label='Target' items={[{content: app.spec.source.targetRevision}]} lightweight />
                </div>
                <Flexy className='application-tile__actions'>
                    <ActionButton label='SYNC' action={() => syncApplication(app.metadata.name)} icon={faSync} />
                    <ActionButton label='REFRESH' action={() => refreshApplication(app.metadata.name)} icon={faRedo} {...refreshLinkAttrs(app)} indicateLoading={true} />
                    <ActionButton label='DELETE' action={() => deleteApplication(app.metadata.name)} icon={faTimesCircle} shouldConfirm={true} indicateLoading={true} />
                </Flexy>
            </div>
        </div>
    );
};
