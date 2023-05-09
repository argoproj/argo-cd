import {DataLoader, Tooltip} from 'argo-ui';
import * as classNames from 'classnames';
import * as React from 'react';
import {Key, KeybindingContext, NumKey, NumKeyToNumber, NumPadKey, useNav} from 'argo-ui/v2';
import {Cluster} from '../../../shared/components';
import {Consumer, Context, AuthSettingsCtx} from '../../../shared/context';
import * as models from '../../../shared/models';
import {ApplicationURLs} from '../application-urls';
import * as AppUtils from '../utils';
// import {getAppDefaultSource, OperationState} from '../utils';
import {services} from '../../../shared/services';

import './applications-tiles.scss';

export interface ApplicationSetTilesProps {
    applicationSets: models.ApplicationSet[];
    // syncApplication: (appName: string, appNamespace: string) => any;
    // refreshApplication: (appName: string, appNamespace: string) => any;
     deleteApplicationSet: (appSetName: string, appSetNamespace: string) => any;
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

export const ApplicationSetTiles = ({applicationSets, deleteApplicationSet}: ApplicationSetTilesProps) => {
    const [selectedAppSet, navApp, reset] = useNav(applicationSets.length);

    const ctxh = React.useContext(Context);
    const appRef = {ref: React.useRef(null), set: false};
    const appContainerRef = React.useRef(null);
    const appsPerRow = useItemsPerContainer(appRef.ref, appContainerRef);
    const useAuthSettingsCtx = React.useContext(AuthSettingsCtx);

    const {useKeybinding} = React.useContext(KeybindingContext);

    useKeybinding({keys: Key.RIGHT, action: () => navApp(1)});
    useKeybinding({keys: Key.LEFT, action: () => navApp(-1)});
    useKeybinding({keys: Key.DOWN, action: () => navApp(appsPerRow)});
    useKeybinding({keys: Key.UP, action: () => navApp(-1 * appsPerRow)});

    useKeybinding({
        keys: Key.ENTER,
        action: () => {
            if (selectedAppSet > -1) {
                ctxh.navigation.goto(`/applicationsets/${applicationSets[selectedAppSet].metadata.name}`);
                return true;
            }
            return false;
        }
    });

    useKeybinding({
        keys: Key.ESCAPE,
        action: () => {
            if (selectedAppSet > -1) {
                reset();
                return true;
            }
            return false;
        }
    });

    useKeybinding({
        keys: Object.values(NumKey) as NumKey[],
        action: n => {
            reset();
            return navApp(NumKeyToNumber(n));
        }
    });
    useKeybinding({
        keys: Object.values(NumPadKey) as NumPadKey[],
        action: n => {
            reset();
            return navApp(NumKeyToNumber(n));
        }
    });
    return (
        <Consumer>
            {ctx => (
                <DataLoader load={() => services.viewPreferences.getPreferences()}>
                    {pref => {
                        const favList = pref.appList.favoritesAppList || [];
                        return (
                            <div
                                className='applications-tiles argo-table-list argo-table-list--clickable row small-up-1 medium-up-2 large-up-3 xxxlarge-up-4'
                                ref={appContainerRef}>
                                {applicationSets.map((appSet, i) => {
                                    // const source = getAppDefaultSource(app);
                                    return (
                                        <div key={AppUtils.appSetInstanceName(appSet)} className='column column-block'>
                                            <div
                                                ref={appRef.set ? null : appRef.ref}
                                                className={`argo-table-list__row applications-list__entry applications-list__entry--health-${appSet.status} ${
                                                    selectedAppSet === i ? 'applications-tiles__selected' : ''
                                                }`}>
                                                <div
                                                    className='row'
                                                    onClick={e =>
                                                        ctx.navigation.goto(
                                                            // `/applicationsets/${appSet.metadata.namespace}/${appSet.metadata.name}`,
                                                            `/applicationsets/${appSet.metadata.name}`,
                                                            {view: pref.appDetails.view},
                                                            {event: e}
                                                        )
                                                    }>
                                                    <div className={`columns small-12 applications-list__info qe-applications-list-${AppUtils.appSetInstanceName(appSet)}`}>
                                                        <div className='row'>
                                                            {/* <div className={app.status.summary.externalURLs?.length > 0 ? 'columns small-10' : 'columns small-11'}> */}
                                                            <div className='columns small-10'>
                                                                {/* <i className={'icon argo-icon-' + (source.chart != null ? 'helm' : 'git')} /> */}
                                                                <i className={'icon argo-icon-git'} />
                                                                <Tooltip content={AppUtils.appSetInstanceName(appSet)}>
                                                                    <span className='applications-list__title'>
                                                                        {AppUtils.appSetQualifiedName(appSet, useAuthSettingsCtx?.appsInAnyNamespaceEnabled)}
                                                                    </span>
                                                                </Tooltip>
                                                            </div>
                                                            {/* <div className={app.status.summary.externalURLs?.length > 0 ? 'columns small-2' : 'columns small-1'}> */}
                                                            <div className='columns small-1'>
                                                                <div className='applications-list__external-link'>
                                                                    {/* <ApplicationURLs urls={app.status.summary.externalURLs} /> */}
                                                                    <Tooltip content={favList?.includes(appSet.metadata.name) ? 'Remove Favorite' : 'Add Favorite'}>
                                                                        <button
                                                                            className='large-text-height'
                                                                            onClick={e => {
                                                                                e.stopPropagation();
                                                                                favList?.includes(appSet.metadata.name)
                                                                                    ? favList.splice(favList.indexOf(appSet.metadata.name), 1)
                                                                                    : favList.push(appSet.metadata.name);
                                                                                services.viewPreferences.updatePreferences({appList: {...pref.appList, favoritesAppList: favList}});
                                                                            }}>
                                                                            <i
                                                                                className={favList?.includes(appSet.metadata.name) ? 'fas fa-star fa-lg' : 'far fa-star fa-lg'}
                                                                                style={{
                                                                                    cursor: 'pointer',
                                                                                    marginLeft: '7px',
                                                                                    color: favList?.includes(appSet.metadata.name) ? '#FFCE25' : '#8fa4b1'
                                                                                }}
                                                                            />
                                                                        </button>
                                                                    </Tooltip>
                                                                </div>
                                                            </div>
                                                                            </div> 
                                                       {/* <div className='row'>
                                                            <div className='columns small-3' title='Project:'>
                                                                Project:
                                                            </div>
                                                            <div className='columns small-9'>{app.spec.project}</div>
                                                        </div> */}
                                                        <div className='row'>
                                                            <div className='columns small-3' title='Labels:'>
                                                                Labels:
                                                            </div>
                                                            <div className='columns small-9'>
                                                                <Tooltip
                                                                    zIndex={4}
                                                                    content={
                                                                        <div>
                                                                            {Object.keys(appSet.metadata.labels || {})
                                                                                .map(label => ({label, value: appSet.metadata.labels[label]}))
                                                                                .map(item => (
                                                                                    <div key={item.label}>
                                                                                        {item.label}={item.value}
                                                                                    </div>
                                                                                ))}
                                                                        </div>
                                                                    }>
                                                                    <span>
                                                                        {Object.keys(appSet.metadata.labels || {})
                                                                            .map(label => `${label}=${appSet.metadata.labels[label]}`)
                                                                            .join(', ')}
                                                                    </span>
                                                                </Tooltip>
                                                            </div>
                                                        </div>
                                                        <div className='row'>
                                                        {/*     <div className='columns small-3' title='Status:'>
                                                                Status:
                                                            </div>
                                                           <div className='columns small-9' qe-id='applications-tiles-health-status'>
                                                                <AppUtils.AppSetHealthStatusIcon state={appSet.status} /> {appSet.status}
                                                                &nbsp;
                                                                <AppUtils.ComparisonStatusIcon status={app.status.sync.status} /> {app.status.sync.status}
                                                                &nbsp;
                                                                <OperationState app={app} quiet={true} />
                                                                
                                                            </div>*/}
                                                        </div>
                                                   {/*     <div className='row'>
                                                            <div className='columns small-3' title='Repository:'>
                                                                Repository:
                                                            </div>
                                                            <div className='columns small-9'>
                                                                <Tooltip content={source.repoURL} zIndex={4}>
                                                                    <span>{source.repoURL}</span>
                                                                </Tooltip>
                                                            </div>
                                                        </div>
                                                        <div className='row'>
                                                            <div className='columns small-3' title='Target Revision:'>
                                                                Target Revision:
                                                            </div>
                                                            <div className='columns small-9'>{source.targetRevision || 'HEAD'}</div>
                                                        </div>
                                                        {source.path && (
                                                            <div className='row'>
                                                                <div className='columns small-3' title='Path:'>
                                                                    Path:
                                                                </div>
                                                                <div className='columns small-9'>{source.path}</div>
                                                            </div>
                                                        )}
                                                        {source.chart && (
                                                            <div className='row'>
                                                                <div className='columns small-3' title='Chart:'>
                                                                    Chart:
                                                                </div>
                                                                <div className='columns small-9'>{source.chart}</div>
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
                                                        */}
                                                        <div className='row'>
                                                            <div className='columns small-3' title='Age:'>
                                                                Created At:
                                                            </div>
                                                            <div className='columns small-9'>{AppUtils.formatCreationTimestamp(appSet.metadata.creationTimestamp)}</div>
                                                        </div>
                                                         {/*
                                                        {app.status.operationState && (
                                                            <div className='row'>
                                                                <div className='columns small-3' title='Last sync:'>
                                                                    Last Sync:
                                                                </div>
                                                                <div className='columns small-9'>
                                                                    {AppUtils.formatCreationTimestamp(app.status.operationState.finishedAt || app.status.operationState.startedAt)}
                                                                </div>
                                                            </div>
                                                        )} 
                                                        */}
                                                        <div className='row'>
                                                            <div className='columns applications-list__entry--actions'>
                                                              {/*   <a
                                                                    className='argo-button argo-button--base'
                                                                    qe-id='applications-tiles-button-sync'
                                                                    onClick={e => {
                                                                        e.stopPropagation();
                                                                        syncApplication(app.metadata.name, app.metadata.namespace);
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
                                                                        refreshApplication(app.metadata.name, app.metadata.namespace);
                                                                    }}>
                                                                    <i className={classNames('fa fa-redo', {'status-icon--spin': AppUtils.isAppRefreshing(app)})} />{' '}
                                                                    <span className='show-for-xxlarge'>Refresh</span>
                                                                </a>
                                                                */}
                                                                &nbsp;
                                                                
                                                                <a
                                                                    className='argo-button argo-button--base'
                                                                    qe-id='applications-tiles-button-delete'
                                                                    onClick={e => {
                                                                        e.stopPropagation();
                                                                        deleteApplicationSet(appSet.metadata.name, appSet.metadata.namespace);
                                                                    }}>
                                                                    <i className='fa fa-times-circle' /> <span className='show-for-xxlarge'>Delete</span>
                                                                </a>
                                                            </div>
                                                        </div>
                                                    </div>
                                                </div>
                                            </div>
                                        </div>
                                    );
                                })}
                            </div>
                        );
                    }}
                </DataLoader>
            )}
        </Consumer>
    );
};
