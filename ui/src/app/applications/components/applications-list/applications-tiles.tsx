import {Tooltip} from 'argo-ui';
import * as classNames from 'classnames';
import * as React from 'react';

import {Consumer, Context} from '../../../shared/context';
import * as models from '../../../shared/models';

import {Cluster} from '../../../shared/components';
import {ApplicationURLs} from '../application-urls';
import * as AppUtils from '../utils';
import {OperationState} from '../utils';
import {Key, useKeyPress} from '../../../shared/keybinding';

require('./applications-tiles.scss');

export interface ApplicationTilesProps {
    applications: models.Application[];
    syncApplication: (appName: string) => any;
    refreshApplication: (appName: string) => any;
    deleteApplication: (appName: string) => any;
}

const useItemsPerContainer = (itemRef: any, containerRef: any): number => {
    const [itemsPer, setItemsPer] = React.useState(0);

    React.useEffect(() => {
        const handleResize = () => {
            let timeoutId: any;
            clearTimeout(timeoutId);
            timeoutId = setTimeout(() => {
                timeoutId = null;
                const itemWidth = itemRef.current ? itemRef.current.offsetWidth : -1;
                const containerWidth = containerRef.current ? containerRef.current.offsetWidth : -1;
                const curItemsPer = containerWidth > 0 && itemWidth > 0 ? Math.floor(containerWidth / itemWidth) : 1;
                if (curItemsPer !== itemsPer) {
                    setItemsPer(curItemsPer);
                }
            }, 1000);
        };
        window.addEventListener('resize', handleResize);
        handleResize();
        return () => {
            window.removeEventListener('resize', handleResize);
        };
    }, []);

    return itemsPer || 1;
};

export const ApplicationTiles = ({applications, syncApplication, refreshApplication, deleteApplication}: ApplicationTilesProps) => {
    const [selectedApp, setSelectedApp] = React.useState(-1);
    const ctxh = React.useContext(Context);
    const appRef = {ref: React.useRef(null), set: false};
    const appContainerRef = React.useRef(null);
    const appsPerRow = useItemsPerContainer(appRef.ref, appContainerRef);

    const isInBounds = (pos: number): boolean => pos < applications.length && pos > -1;

    const nav = (val: number): boolean => {
        const newPos = selectedApp + val;
        return isInBounds(newPos) ? setSelectedApp(newPos) === null : false;
    };

    useKeyPress(Key.RIGHT, () => nav(1));
    useKeyPress(Key.LEFT, () => nav(-1));
    useKeyPress(Key.DOWN, () => nav(appsPerRow));
    useKeyPress(Key.UP, () => nav(-1 * appsPerRow));

    useKeyPress(Key.ENTER, () => {
        if (selectedApp > -1) {
            ctxh.navigation.goto(`/applications/${applications[selectedApp].metadata.name}`);
            return true;
        }
        return false;
    });

    useKeyPress(Key.ESCAPE, () => {
        if (selectedApp > -1) {
            setSelectedApp(-1);
            return true;
        }
        return false;
    });

    return (
        <Consumer>
            {ctx => (
                <div className='applications-tiles argo-table-list argo-table-list--clickable row small-up-1 medium-up-2 large-up-3 xxxlarge-up-4' ref={appContainerRef}>
                    {applications.map((app, i) => (
                        <div key={app.metadata.name} className='column column-block'>
                            <div
                                ref={appRef.set ? null : appRef.ref}
                                className={`argo-table-list__row applications-list__entry applications-list__entry--comparison-${
                                    app.status.sync.status
                                } applications-list__entry--health-${app.status.health.status} ${selectedApp === i ? 'applications-tiles__selected' : ''}`}>
                                <div className='row' onClick={e => ctx.navigation.goto(`/applications/${app.metadata.name}`, {}, {event: e})}>
                                    <div className={`columns small-12 applications-list__info qe-applications-list-${app.metadata.name}`}>
                                        <div className='applications-list__external-link'>
                                            <ApplicationURLs urls={app.status.summary.externalURLs} />
                                        </div>
                                        <div className='row'>
                                            <div className='columns small-12'>
                                                <i className={'icon argo-icon-' + (app.spec.source.chart != null ? 'helm' : 'git')} />
                                                <span className='applications-list__title'>{app.metadata.name}</span>
                                            </div>
                                        </div>
                                        <div className='row'>
                                            <div className='columns small-3' title='Project:'>
                                                Project:
                                            </div>
                                            <div className='columns small-9'>{app.spec.project}</div>
                                        </div>
                                        <div className='row'>
                                            <div className='columns small-3' title='Labels:'>
                                                Labels:
                                            </div>
                                            <div className='columns small-9'>
                                                <Tooltip
                                                    content={
                                                        <div>
                                                            {Object.keys(app.metadata.labels || {})
                                                                .map(label => ({label, value: app.metadata.labels[label]}))
                                                                .map(item => (
                                                                    <div key={item.label}>
                                                                        {item.label}={item.value}
                                                                    </div>
                                                                ))}
                                                        </div>
                                                    }>
                                                    <span>
                                                        {Object.keys(app.metadata.labels || {})
                                                            .map(label => `${label}=${app.metadata.labels[label]}`)
                                                            .join(', ')}
                                                    </span>
                                                </Tooltip>
                                            </div>
                                        </div>
                                        <div className='row'>
                                            <div className='columns small-3' title='Status:'>
                                                Status:
                                            </div>
                                            <div className='columns small-9' qe-id='applications-tiles-health-status'>
                                                <AppUtils.HealthStatusIcon state={app.status.health} /> {app.status.health.status}
                                                &nbsp;
                                                <AppUtils.ComparisonStatusIcon status={app.status.sync.status} /> {app.status.sync.status}
                                                &nbsp;
                                                <OperationState app={app} quiet={true} />
                                            </div>
                                        </div>
                                        <div className='row'>
                                            <div className='columns small-3' title='Repository:'>
                                                Repository:
                                            </div>
                                            <div className='columns small-9'>
                                                <Tooltip content={app.spec.source.repoURL}>
                                                    <span>{app.spec.source.repoURL}</span>
                                                </Tooltip>
                                            </div>
                                        </div>
                                        <div className='row'>
                                            <div className='columns small-3' title='Target Revision:'>
                                                Target Revision:
                                            </div>
                                            <div className='columns small-9'>{app.spec.source.targetRevision}</div>
                                        </div>
                                        {app.spec.source.path && (
                                            <div className='row'>
                                                <div className='columns small-3' title='Path:'>
                                                    Path:
                                                </div>
                                                <div className='columns small-9'>{app.spec.source.path}</div>
                                            </div>
                                        )}
                                        {app.spec.source.chart && (
                                            <div className='row'>
                                                <div className='columns small-3' title='Chart:'>
                                                    Chart:
                                                </div>
                                                <div className='columns small-9'>{app.spec.source.chart}</div>
                                            </div>
                                        )}
                                        <div className='row'>
                                            <div className='columns small-3' title='Destination:'>
                                                Destination:
                                            </div>
                                            <div className='columns small-9'>
                                                <Cluster server={app.spec.destination.server} name={app.spec.destination.name} />
                                            </div>
                                        </div>
                                        <div className='row'>
                                            <div className='columns small-3' title='Namespace:'>
                                                Namespace:
                                            </div>
                                            <div className='columns small-9'>{app.spec.destination.namespace}</div>
                                        </div>
                                        <div className='row'>
                                            <div className='columns applications-list__entry--actions'>
                                                <a
                                                    className='argo-button argo-button--base'
                                                    qe-id='applications-tiles-button-sync'
                                                    onClick={e => {
                                                        e.stopPropagation();
                                                        syncApplication(app.metadata.name);
                                                    }}>
                                                    <i className='fa fa-sync' /> Sync
                                                </a>
                                                &nbsp;
                                                <a
                                                    className='argo-button argo-button--base'
                                                    qe-id='applications-tiles-button-refresh'
                                                    {...AppUtils.refreshLinkAttrs(app)}
                                                    onClick={e => {
                                                        e.stopPropagation();
                                                        refreshApplication(app.metadata.name);
                                                    }}>
                                                    <i className={classNames('fa fa-redo', {'status-icon--spin': AppUtils.isAppRefreshing(app)})} />{' '}
                                                    <span className='show-for-xlarge'>Refresh</span>
                                                </a>
                                                &nbsp;
                                                <a
                                                    className='argo-button argo-button--base'
                                                    qe-id='applications-tiles-button-delete'
                                                    onClick={e => {
                                                        e.stopPropagation();
                                                        deleteApplication(app.metadata.name);
                                                    }}>
                                                    <i className='fa fa-times-circle' /> Delete
                                                </a>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    ))}
                </div>
            )}
        </Consumer>
    );
};
