import {DataLoader, Tooltip} from 'argo-ui';
import * as classNames from 'classnames';
import * as React from 'react';
import {Key, KeybindingContext, NumKey, NumKeyToNumber, NumPadKey, useNav} from 'argo-ui/v2';
import {Cluster} from '../../../shared/components';
import {Consumer, Context, AuthSettingsCtx} from '../../../shared/context';
import * as models from '../../../shared/models';
import {ApplicationURLs} from '../application-urls';
import * as AppUtils from '../utils';
import {getAppDefaultSource, OperationState, getApplicationLinkURL, getManagedByURL, isApp, getAppSetHealthStatus} from '../utils';
import {services} from '../../../shared/services';

import './applications-tiles.scss';

export interface ApplicationTilesProps {
    applications: models.AbstractApplication[];
    syncApplication: (appName: string, appNamespace: string) => any;
    refreshApplication: (appName: string, appNamespace: string) => any;
    deleteApplication: (appName: string, appNamespace: string) => any;
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
    const [selectedApp, navApp, reset] = useNav(applications.length);

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
            if (selectedApp > -1) {
                ctxh.navigation.goto(`/${AppUtils.getAppUrl(applications[selectedApp])}`);
                return true;
            }
            return false;
        }
    });

    useKeybinding({
        keys: Key.ESCAPE,
        action: () => {
            if (selectedApp > -1) {
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
                            <div className='applications-tiles argo-table-list argo-table-list--clickable' ref={appContainerRef}>
                                {applications.map((app, i) => {
                                    const isApplication = isApp(app);
                                    const typedApp = isApplication ? (app as models.Application) : null;
                                    const typedAppSet = !isApplication ? (app as models.ApplicationSet) : null;
                                    const source = isApplication ? getAppDefaultSource(typedApp) : null;
                                    const isOci = source?.repoURL?.startsWith('oci://');
                                    const targetRevision = source ? source.targetRevision || 'HEAD' : 'Unknown';
                                    const linkInfo = getApplicationLinkURL(app, ctx.baseHref);
                                    const healthStatus = isApplication ? typedApp.status.health.status : getAppSetHealthStatus(typedAppSet);
                                    return (
                                        <div
                                            key={AppUtils.appInstanceName(app)}
                                            ref={appRef.set ? null : appRef.ref}
                                            className={`argo-table-list__row applications-list__entry applications-list__entry--health-${healthStatus} ${
                                                selectedApp === i ? 'applications-tiles__selected' : ''
                                            }`}>
                                            <div
                                                className='row applications-tiles__wrapper'
                                                onClick={e => ctx.navigation.goto(`/${AppUtils.getAppUrl(app)}`, {view: pref.appDetails.view}, {event: e})}>
                                                <div
                                                    className={`columns small-12 applications-list__info qe-applications-list-${AppUtils.appInstanceName(
                                                        app
                                                    )} applications-tiles__item`}>
                                                    <div className='row '>
                                                        {isApplication && (
                                                            <div className={typedApp.status.summary?.externalURLs?.length > 0 ? 'columns small-10' : 'columns small-11'}>
                                                                <i
                                                                    className={
                                                                        'icon argo-icon-' + (source?.chart != null ? 'helm' : isOci ? 'oci applications-tiles__item__small' : 'git')
                                                                    }
                                                                />
                                                                <Tooltip content={AppUtils.appInstanceName(app)}>
                                                                    <span className='applications-list__title'>
                                                                        {AppUtils.appQualifiedName(app, useAuthSettingsCtx?.appsInAnyNamespaceEnabled)}
                                                                    </span>
                                                                </Tooltip>
                                                            </div>
                                                        )}
                                                        {!isApplication && (
                                                            <div className='columns small-11'>
                                                                <i className='icon argo-icon-git' />
                                                                <Tooltip content={AppUtils.appInstanceName(app)}>
                                                                    <span className='applications-list__title'>
                                                                        {AppUtils.appQualifiedName(app, useAuthSettingsCtx?.appsInAnyNamespaceEnabled)}
                                                                    </span>
                                                                </Tooltip>
                                                            </div>
                                                        )}
                                                        <div className={isApplication && typedApp.status.summary?.externalURLs?.length > 0 ? 'columns small-2' : 'columns small-1'}>
                                                            <div className='applications-list__external-link'>
                                                                {isApplication && <ApplicationURLs urls={typedApp.status.summary?.externalURLs} />}
                                                                <button
                                                                    onClick={e => {
                                                                        e.stopPropagation();
                                                                        if (linkInfo.isExternal) {
                                                                            window.open(linkInfo.url, '_blank', 'noopener,noreferrer');
                                                                        } else {
                                                                            ctx.navigation.goto(`/${AppUtils.getAppUrl(app)}`);
                                                                        }
                                                                    }}
                                                                    title={getManagedByURL(app) ? `Managed by: ${getManagedByURL(app)}` : 'Open application'}>
                                                                    <i className='fa fa-external-link-alt' />
                                                                </button>
                                                                <button
                                                                    title={favList?.includes(app.metadata.name) ? 'Remove Favorite' : 'Add Favorite'}
                                                                    className='large-text-height'
                                                                    onClick={e => {
                                                                        e.stopPropagation();
                                                                        favList?.includes(app.metadata.name)
                                                                            ? favList.splice(favList.indexOf(app.metadata.name), 1)
                                                                            : favList.push(app.metadata.name);
                                                                        services.viewPreferences.updatePreferences({appList: {...pref.appList, favoritesAppList: favList}});
                                                                    }}>
                                                                    <i
                                                                        className={favList?.includes(app.metadata.name) ? 'fas fa-star fa-lg' : 'far fa-star fa-lg'}
                                                                        style={{
                                                                            cursor: 'pointer',
                                                                            margin: '-1px 0px 0px 7px',
                                                                            color: favList?.includes(app.metadata.name) ? '#FFCE25' : '#8fa4b1'
                                                                        }}
                                                                    />
                                                                </button>
                                                            </div>
                                                        </div>
                                                    </div>
                                                    {isApplication && (
                                                        <div className='row'>
                                                            <div className='columns small-3' title='Project:'>
                                                                Project:
                                                            </div>
                                                            <div className='columns small-9'>{typedApp.spec.project}</div>
                                                        </div>
                                                    )}
                                                    <div className='row'>
                                                        <div className='columns small-3' title='Labels:'>
                                                            Labels:
                                                        </div>
                                                        <div className='columns small-9'>
                                                            <Tooltip
                                                                zIndex={4}
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
                                                        <div className='columns small-3' title='Annotations:'>
                                                            Annotations:
                                                        </div>
                                                        <div className='columns small-9'>
                                                            <Tooltip
                                                                zIndex={4}
                                                                content={
                                                                    <div>
                                                                        {Object.keys(app.metadata.annotations || {})
                                                                            .map(annotation => ({label: annotation, value: app.metadata.annotations[annotation]}))
                                                                            .map(item => (
                                                                                <div key={item.label}>
                                                                                    {item.label}={item.value}
                                                                                </div>
                                                                            ))}
                                                                    </div>
                                                                }>
                                                                <span>
                                                                    {Object.keys(app.metadata.annotations || {})
                                                                        .map(annotation => `${annotation}=${app.metadata.annotations[annotation]}`)
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
                                                            {isApplication ? (
                                                                <>
                                                                    <AppUtils.HealthStatusIcon state={typedApp.status.health} /> {typedApp.status.health.status}
                                                                    &nbsp;
                                                                    {typedApp.status.sourceHydrator?.currentOperation && (
                                                                        <>
                                                                            <AppUtils.HydrateOperationPhaseIcon operationState={typedApp.status.sourceHydrator.currentOperation} />{' '}
                                                                            {typedApp.status.sourceHydrator.currentOperation.phase}
                                                                            &nbsp;
                                                                        </>
                                                                    )}
                                                                    <AppUtils.ComparisonStatusIcon status={typedApp.status.sync.status} /> {typedApp.status.sync.status}
                                                                    &nbsp;
                                                                    <OperationState app={typedApp} quiet={true} />
                                                                </>
                                                            ) : (
                                                                <>
                                                                    <AppUtils.HealthStatusIcon state={{status: healthStatus, message: ''}} /> {healthStatus}
                                                                </>
                                                            )}
                                                        </div>
                                                    </div>
                                                    {isApplication && (
                                                        <>
                                                            <div className='row'>
                                                                <div className='columns small-3' title='Repository:'>
                                                                    Repository:
                                                                </div>
                                                                <div className='columns small-9'>
                                                                    <Tooltip content={source?.repoURL || ''} zIndex={4}>
                                                                        <span>{source?.repoURL}</span>
                                                                    </Tooltip>
                                                                </div>
                                                            </div>
                                                            <div className='row'>
                                                                <div className='columns small-3' title='Target Revision:'>
                                                                    Target Revision:
                                                                </div>
                                                                <div className='columns small-9'>{targetRevision}</div>
                                                            </div>
                                                            {source?.path && (
                                                                <div className='row'>
                                                                    <div className='columns small-3' title='Path:'>
                                                                        Path:
                                                                    </div>
                                                                    <div className='columns small-9'>{source?.path}</div>
                                                                </div>
                                                            )}
                                                            {source?.chart && (
                                                                <div className='row'>
                                                                    <div className='columns small-3' title='Chart:'>
                                                                        Chart:
                                                                    </div>
                                                                    <div className='columns small-9'>{source?.chart}</div>
                                                                </div>
                                                            )}
                                                            <div className='row'>
                                                                <div className='columns small-3' title='Destination:'>
                                                                    Destination:
                                                                </div>
                                                                <div className='columns small-9'>
                                                                    <Cluster server={typedApp.spec.destination.server} name={typedApp.spec.destination.name} />
                                                                </div>
                                                            </div>
                                                            <div className='row'>
                                                                <div className='columns small-3' title='Namespace:'>
                                                                    Namespace:
                                                                </div>
                                                                <div className='columns small-9'>{typedApp.spec.destination.namespace}</div>
                                                            </div>
                                                        </>
                                                    )}
                                                    {!isApplication && typedAppSet && (
                                                        <div className='row'>
                                                            <div className='columns small-3' title='Applications:'>
                                                                Applications:
                                                            </div>
                                                            <div className='columns small-9'>
                                                                {typedAppSet.status?.resourcesCount ?? typedAppSet.status?.resources?.length ?? 0}
                                                            </div>
                                                        </div>
                                                    )}
                                                    <div className='row'>
                                                        <div className='columns small-3' title='Age:'>
                                                            Created At:
                                                        </div>
                                                        <div className='columns small-9'>{AppUtils.formatCreationTimestamp(app.metadata.creationTimestamp)}</div>
                                                    </div>
                                                    {isApplication && typedApp.status.operationState && (
                                                        <div className='row'>
                                                            <div className='columns small-3' title='Last sync:'>
                                                                Last Sync:
                                                            </div>
                                                            <div className='columns small-9'>
                                                                {AppUtils.formatCreationTimestamp(
                                                                    typedApp.status.operationState.finishedAt || typedApp.status.operationState.startedAt
                                                                )}
                                                            </div>
                                                        </div>
                                                    )}
                                                    {isApplication && (
                                                        <div className='row applications-tiles__actions'>
                                                            <div className='columns applications-list__entry--actions'>
                                                                <a
                                                                    className='argo-button argo-button--base'
                                                                    qe-id='applications-tiles-button-sync'
                                                                    onClick={e => {
                                                                        e.stopPropagation();
                                                                        syncApplication(app.metadata.name, app.metadata.namespace);
                                                                    }}>
                                                                    <i className='fa fa-sync' /> Sync
                                                                </a>
                                                                &nbsp;
                                                                <Tooltip className='custom-tooltip' content={'Refresh'}>
                                                                    <a
                                                                        className='argo-button argo-button--base'
                                                                        qe-id='applications-tiles-button-refresh'
                                                                        {...AppUtils.refreshLinkAttrs(typedApp)}
                                                                        onClick={e => {
                                                                            e.stopPropagation();
                                                                            refreshApplication(app.metadata.name, app.metadata.namespace);
                                                                        }}>
                                                                        <i className={classNames('fa fa-redo', {'status-icon--spin': AppUtils.isAppRefreshing(typedApp)})} />{' '}
                                                                        <span className='show-for-xxlarge'>Refresh</span>
                                                                    </a>
                                                                </Tooltip>
                                                                &nbsp;
                                                                <Tooltip className='custom-tooltip' content={'Delete'}>
                                                                    <a
                                                                        className='argo-button argo-button--base'
                                                                        qe-id='applications-tiles-button-delete'
                                                                        onClick={e => {
                                                                            e.stopPropagation();
                                                                            deleteApplication(app.metadata.name, app.metadata.namespace);
                                                                        }}>
                                                                        <i className='fa fa-times-circle' /> <span className='show-for-xxlarge'>Delete</span>
                                                                    </a>
                                                                </Tooltip>
                                                            </div>
                                                        </div>
                                                    )}
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
