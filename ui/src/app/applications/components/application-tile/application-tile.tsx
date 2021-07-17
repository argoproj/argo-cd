import {ActionButton, Checkbox, Flexy, InfoItemRow, useData} from 'argo-ui/v2';
import * as React from 'react';
import {ClusterCtx, clusterTitle, GetCluster} from '../../../shared/components';
import {Context} from '../../../shared/context';
import {Application} from '../../../shared/models';
import {ApplicationURLs} from '../application-urls';
import {ComparisonStatusIcon, HealthStatusIcon, refreshLinkAttrs} from '../utils';
import './application-tile.scss';

type AppAction = (app: string) => void;

interface TileProps {
    app: Application;
    selected?: boolean;
    checked?: boolean;
    ref: React.MutableRefObject<any>;
    onSelect?: (selected: boolean) => void;
    syncApplication: AppAction;
    refreshApplication: AppAction;
    deleteApplication: AppAction;
    refreshing?: boolean;
    compact?: boolean;
}

const TileActions = (props: TileProps) => {
    const {app, compact, syncApplication, refreshApplication, deleteApplication, refreshing} = props;
    return (
        <Flexy className={`application-tile__actions ${props.compact ? 'application-tile__actions--compact' : ''}`}>
            <ActionButton label='SYNC' action={() => syncApplication(app.metadata.name)} icon='fa-sync' short={compact} transparent={compact} tooltip={compact && 'SYNC'} />
            <ActionButton
                label='REFRESH'
                action={() => refreshApplication(app.metadata.name)}
                icon='fa-redo'
                {...refreshLinkAttrs(app)}
                indicateLoading={true}
                short={compact}
                transparent={compact}
                loading={refreshing}
                tooltip={compact && 'REFRESH'}
            />
            <ActionButton
                label='DELETE'
                action={() => deleteApplication(app.metadata.name)}
                icon='fa-times-circle'
                shouldConfirm={true}
                indicateLoading={true}
                short={compact}
                transparent={compact}
                tooltip={compact && 'DELETE'}
            />
        </Flexy>
    );
};

export const ApplicationTile = (props: TileProps) => {
    const {app, selected, ref, compact} = props;
    const appContext = React.useContext(Context);
    const clusterContext = React.useContext(ClusterCtx);
    const [cluster, loading] = useData(() => GetCluster(clusterContext, app.spec.destination.server, app.spec.destination.name));

    return (
        <div key={app.metadata.name} className='column column-block'>
            <div
                ref={ref}
                className={`application-tile ${compact ? 'application-tile--compact' : ''} argo-table-list__row applications-list__entry applications-list__entry--comparison-${
                    app.status.sync.status
                } applications-list__entry--health-${app.status.health.status} ${selected ? 'applications-tiles__selected' : ''}`}>
                {compact && <TileActions {...props} />}
                <div onClick={e => appContext.navigation.goto(`/applications/${app.metadata.name}`, {}, {event: e})} style={{minWidth: 0, flexGrow: 1}}>
                    <Flexy className='application-tile__header'>
                        <Checkbox
                            onChange={val => {
                                if (props.onSelect) {
                                    props.onSelect(val);
                                }
                            }}
                            value={props.checked}
                        />
                        {app.metadata.name}
                        <div style={{marginLeft: 'auto'}}>
                            <ApplicationURLs urls={app.status.summary.externalURLs} />
                            <i className={app.spec.source.chart != null ? 'icon argo-icon-helm' : 'fab fa-git-square'} style={{marginRight: '7px'}} />
                            <span style={{marginRight: '7px'}}>
                                <HealthStatusIcon state={app.status.health} />
                            </span>
                            <span style={{marginRight: '7px'}}>
                                <ComparisonStatusIcon status={app.status.sync.status} />
                            </span>
                        </div>
                    </Flexy>
                    <InfoItemRow label='Namespace' items={[{content: app.spec.destination.namespace}]} lightweight={true} />
                    <InfoItemRow label='Project' items={[{content: app.spec.project}]} lightweight={true} />
                    {!props.compact && (
                        <React.Fragment>
                            <InfoItemRow label='Repo' items={[{content: app.spec.source.repoURL, truncate: true}]} lightweight={true} />
                            <InfoItemRow label='Destination' items={[{content: loading ? 'Loading...' : clusterTitle(cluster), truncate: true}]} lightweight={true} />
                            <InfoItemRow
                                label='Labels'
                                items={
                                    app.metadata.labels && Object.keys(app.metadata.labels).length > 0
                                        ? Object.keys(app.metadata.labels).map(l => {
                                              return {content: `${l}=${app.metadata.labels[l]}`};
                                          })
                                        : [{content: 'None', lightweight: true}]
                                }
                            />
                            <InfoItemRow label='Path' items={[{content: app.spec.source.path}]} lightweight={true} />
                            <InfoItemRow label='Target' items={[{content: app.spec.source.targetRevision}]} lightweight={true} />
                        </React.Fragment>
                    )}
                </div>
                {!compact && <TileActions {...props} />}
            </div>
        </div>
    );
};
