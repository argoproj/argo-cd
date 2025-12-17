import {DropDownMenu, NotificationType, SlidingPanel, Tooltip} from 'argo-ui';
import * as classNames from 'classnames';
import React, {useState, useEffect, useCallback, useRef, useContext, FC} from 'react';
import * as ReactDOM from 'react-dom';
import * as models from '../../../shared/models';
import {RouteComponentProps} from 'react-router';
import {BehaviorSubject, combineLatest, from, merge, Observable} from 'rxjs';
import {delay, filter, map, mergeMap, repeat, retryWhen} from 'rxjs/operators';

import {DataLoader, EmptyState, ErrorNotification, ObservableQuery, Page, Paginate, Revision, Timestamp} from '../../../shared/components';
import {AppContext, Context, ContextApis} from '../../../shared/context';
import * as appModels from '../../../shared/models';
import {AppDetailsPreferences, AppsDetailsViewKey, AppsDetailsViewType, services} from '../../../shared/services';

import {ApplicationConditions} from '../application-conditions/application-conditions';
import {ApplicationDeploymentHistory} from '../application-deployment-history/application-deployment-history';
import {ApplicationOperationState} from '../application-operation-state/application-operation-state';
import {PodGroupType, PodView} from '../application-pod-view/pod-view';
import {ApplicationResourceTree, ResourceTreeNode} from '../application-resource-tree/application-resource-tree';
import {ApplicationStatusPanel} from '../application-status-panel/application-status-panel';
import {ApplicationSyncPanel} from '../application-sync-panel/application-sync-panel';
import {ResourceDetails} from '../resource-details/resource-details';
import * as AppUtils from '../utils';
import {ApplicationResourceList} from './application-resource-list';
import {Filters, FiltersProps} from './application-resource-filter';
import {getAppDefaultSource, getAppCurrentVersion, urlPattern} from '../utils';
import {ChartDetails, OCIMetadata, ResourceStatus} from '../../../shared/models';
import {ApplicationsDetailsAppDropdown} from './application-details-app-dropdown';
import {useSidebarTarget} from '../../../sidebar/sidebar';

import './application-details.scss';
import {TopBarActionMenuExt, AppViewExtension, StatusPanelExtension} from '../../../shared/services/extensions-service';
import {ApplicationHydrateOperationState} from '../application-hydrate-operation-state/application-hydrate-operation-state';

interface ApplicationDetailsState {
    page: number;
    revision?: string; // Which type of revision panelto show SYNC_STATUS_REVISION or OPERATION_STATE_REVISION
    groupedResources?: ResourceStatus[];
    slidingPanelPage?: number;
    filteredGraph?: any[];
    truncateNameOnRight?: boolean;
    showFullNodeName?: boolean;
    collapsedNodes?: string[];
    extensions?: AppViewExtension[];
    extensionsMap?: {[key: string]: AppViewExtension};
    statusExtensions?: StatusPanelExtension[];
    statusExtensionsMap?: {[key: string]: StatusPanelExtension};
    topBarActionMenuExts?: TopBarActionMenuExt[];
    topBarActionMenuExtsMap?: {[key: string]: TopBarActionMenuExt};
}

interface FilterInput {
    name: string[];
    kind: string[];
    health: string[];
    sync: string[];
    namespace: string[];
}

const ApplicationDetailsFilters = (props: FiltersProps) => {
    const sidebarTarget = useSidebarTarget();
    return ReactDOM.createPortal(<Filters {...props} />, sidebarTarget?.current);
};

export const NodeInfo = (node?: string): {key: string; container: number} => {
    const nodeContainer = {key: '', container: 0};
    if (node) {
        const parts = node.split('/');
        nodeContainer.key = parts.slice(0, 4).join('/');
        nodeContainer.container = parseInt(parts[4] || '0', 10);
    }
    return nodeContainer;
};

export const SelectNode = (fullName: string, containerIndex = 0, tab: string = null, appContext: ContextApis) => {
    const node = fullName ? `${fullName}/${containerIndex}` : null;
    appContext.navigation.goto('.', {node, tab}, {replace: true});
};

export const ApplicationDetails: FC<RouteComponentProps<{appnamespace: string; name: string}> & {objectListKind: string}> = props => {
    const appContext = useContext(Context);
    const appChanged = useRef(new BehaviorSubject<appModels.AbstractApplication>(null));
    const objectListKind = props.objectListKind;

    const getExtensionsState = useCallback(() => {
        const extensions = services.extensions.getAppViewExtensions();
        const extensionsMap: {[key: string]: AppViewExtension} = {};
        extensions.forEach(ext => {
            extensionsMap[ext.title] = ext;
        });
        const statusExtensions = services.extensions.getStatusPanelExtensions();
        const statusExtensionsMap: {[key: string]: StatusPanelExtension} = {};
        statusExtensions.forEach(ext => {
            statusExtensionsMap[ext.id] = ext;
        });
        const topBarActionMenuExts = services.extensions.getActionMenuExtensions();
        const topBarActionMenuExtsMap: {[key: string]: TopBarActionMenuExt} = {};
        topBarActionMenuExts.forEach(ext => {
            topBarActionMenuExtsMap[ext.id] = ext;
        });
        return {extensions, extensionsMap, statusExtensions, statusExtensionsMap, topBarActionMenuExts, topBarActionMenuExtsMap};
    }, []);

    const [state, setState] = useState<ApplicationDetailsState>(() => ({
        page: 0,
        groupedResources: [],
        slidingPanelPage: 0,
        filteredGraph: [],
        truncateNameOnRight: false,
        showFullNodeName: false,
        collapsedNodes: [],
        ...getExtensionsState()
    }));

    const getAppNamespace = useCallback(() => {
        if (typeof props.match.params.appnamespace === 'undefined') {
            return '';
        }
        return props.match.params.appnamespace;
    }, [props.match.params.appnamespace]);

    const onExtensionsUpdate = useCallback(() => {
        setState(prevState => ({...prevState, ...getExtensionsState()}));
    }, [getExtensionsState]);

    useEffect(() => {
        services.extensions.addEventListener('resource', onExtensionsUpdate);
        services.extensions.addEventListener('appView', onExtensionsUpdate);
        services.extensions.addEventListener('statusPanel', onExtensionsUpdate);
        services.extensions.addEventListener('topBar', onExtensionsUpdate);

        return () => {
            services.extensions.removeEventListener('resource', onExtensionsUpdate);
            services.extensions.removeEventListener('appView', onExtensionsUpdate);
            services.extensions.removeEventListener('statusPanel', onExtensionsUpdate);
            services.extensions.removeEventListener('topBar', onExtensionsUpdate);
        };
    }, [onExtensionsUpdate]);

    const showHydrateOperationState = new URLSearchParams(props.history.location.search).get('hydrateOperation') === 'true';
    const showOperationState = new URLSearchParams(props.history.location.search).get('operation') === 'true';
    const showConditions = new URLSearchParams(props.history.location.search).get('conditions') === 'true';
    const selectedRollbackDeploymentIndex = parseInt(new URLSearchParams(props.history.location.search).get('rollback'), 10);
    const selectedNodeInfo = NodeInfo(new URLSearchParams(props.history.location.search).get('node'));
    const selectedNodeKey = selectedNodeInfo.key;
    const selectedExtension = new URLSearchParams(props.history.location.search).get('extension');

    // Define escapeRegex first as it's used by other functions
    const escapeRegex = useCallback((input: string): string => {
        return input.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    }, []);

    // Define nodeNameMatchesWildcardFilters as it's used by filterTreeNode
    const nodeNameMatchesWildcardFilters = useCallback(
        (nodeName: string, filterInputNames: string[]): boolean => {
            const regularExpression = new RegExp(
                filterInputNames
                    // Escape any regex input to ensure only * can be used
                    .map(pattern => '^' + escapeRegex(pattern) + '$')
                    // Replace any escaped * with proper regex
                    .map(pattern => pattern.replace(/\\\*/g, '.*'))
                    // Join all filterInputs to a single regular expression
                    .join('|'),
                'gi'
            );
            return regularExpression.test(nodeName);
        },
        [escapeRegex]
    );

    const setNodeExpansion = useCallback((node: string, isExpanded: boolean) => {
        setState(prevState => {
            const index = prevState.collapsedNodes.indexOf(node);
            const updatedNodes = [...prevState.collapsedNodes];

            if (isExpanded && index >= 0) {
                updatedNodes.splice(index, 1);
            } else if (!isExpanded && index < 0) {
                updatedNodes.push(node);
            }

            return {...prevState, collapsedNodes: updatedNodes};
        });
    }, []);

    const getNodeExpansion = useCallback(
        (node: string): boolean => {
            return state.collapsedNodes.indexOf(node) < 0;
        },
        [state.collapsedNodes]
    );

    const closeGroupedNodesPanel = useCallback(() => {
        setState(prevState => ({
            ...prevState,
            groupedResources: [],
            slidingPanelPage: 0
        }));
    }, []);

    const toggleCompactView = useCallback((appName: string, pref: AppDetailsPreferences) => {
        pref.userHelpTipMsgs = pref.userHelpTipMsgs.map(usrMsg => (usrMsg.appName === appName && usrMsg.msgKey === 'groupNodes' ? {...usrMsg, display: true} : usrMsg));
        services.viewPreferences.updatePreferences({appDetails: {...pref, groupNodes: !pref.groupNodes}});
    }, []);

    const setOperationStatusVisible = useCallback(
        (isVisible: boolean) => {
            appContext.navigation.goto('.', {operation: isVisible}, {replace: true});
        },
        [appContext]
    );

    const setHydrateOperationStatusVisible = useCallback(
        (isVisible: boolean) => {
            appContext.navigation.goto('.', {hydrateOperation: isVisible}, {replace: true});
        },
        [appContext]
    );

    const setConditionsStatusVisible = useCallback(
        (isVisible: boolean) => {
            appContext.navigation.goto('.', {conditions: isVisible}, {replace: true});
        },
        [appContext]
    );

    const setRollbackPanelVisible = useCallback(
        (selectedDeploymentIndex = 0) => {
            appContext.navigation.goto('.', {rollback: selectedDeploymentIndex}, {replace: true});
        },
        [appContext]
    );

    const setExtensionPanelVisible = useCallback(
        (selectedExtension = '') => {
            appContext.navigation.goto('.', {extension: selectedExtension}, {replace: true});
        },
        [appContext]
    );

    const selectNode = useCallback(
        (fullName: string, containerIndex = 0, tab: string = null) => {
            SelectNode(fullName, containerIndex, tab, appContext);
        },
        [appContext]
    );

    const deleteApplication = useCallback(async () => {
        await AppUtils.deleteApplication(props.match.params.name, getAppNamespace(), appContext);
    }, [props.match.params.name, getAppNamespace, appContext]);

    const confirmDeletion = useCallback(
        async (app: appModels.Application, title: string, message: string) => {
            const confirmed = await appContext.popup.confirm(title, message);
            if (confirmed) {
                if (!app.metadata.annotations) {
                    app.metadata.annotations = {};
                }
                app.metadata.annotations[appModels.AppDeletionConfirmedAnnotation] = new Date().toISOString();
                await services.applications.update(app);
            }
        },
        [appContext]
    );

    const rollbackApplication = useCallback(
        async (revisionHistory: appModels.RevisionHistory, application: appModels.Application) => {
            try {
                const needDisableRollback = application.spec.syncPolicy && application.spec.syncPolicy.automated;
                let confirmationMessage = `Are you sure you want to rollback application '${props.match.params.name}'?`;
                if (needDisableRollback) {
                    confirmationMessage = `Auto-Sync needs to be disabled in order for rollback to occur.
Are you sure you want to disable auto-sync and rollback application '${props.match.params.name}'?`;
                }

                const confirmed = await appContext.popup.confirm('Rollback application', confirmationMessage);
                if (confirmed) {
                    if (needDisableRollback) {
                        const update = JSON.parse(JSON.stringify(application)) as appModels.Application;
                        update.spec.syncPolicy.automated = null;
                        await services.applications.update(update, {validate: false});
                    }
                    await services.applications.rollback(props.match.params.name, getAppNamespace(), revisionHistory.id);
                    appChanged.current.next(await services.applications.get(props.match.params.name, getAppNamespace(), objectListKind));
                    setRollbackPanelVisible(-1);
                }
            } catch (e) {
                appContext.notifications.show({
                    content: <ErrorNotification title='Unable to rollback application' e={e} />,
                    type: NotificationType.Error
                });
            }
        },
        [props.match.params.name, getAppNamespace, appContext, objectListKind]
    );

    const getPageTitle = useCallback((view: string) => {
        const {Tree, Pods, Network, List} = AppsDetailsViewKey;
        switch (view) {
            case Tree:
                return 'Application Details Tree';
            case Network:
                return 'Application Details Network';
            case Pods:
                return 'Application Details Pods';
            case List:
                return 'Application Details List';
        }
        return '';
    }, []);

    const getContent = useCallback((application: models.Application, source: models.ApplicationSource, revisions: string[], revision: string) => {
        const renderCommitMessage = (message: string) =>
            message.split(/\s/).map(part =>
                urlPattern.test(part) ? (
                    <a href={part} target='_blank' rel='noopener noreferrer' style={{overflowWrap: 'anywhere', wordBreak: 'break-word'}}>
                        {part}{' '}
                    </a>
                ) : (
                    part + ' '
                )
            );

        const getContentForOci = (
            aRevision: string,
            aSourceIndex: number | null,
            aVersionId: number | null,
            indx: number,
            aSource: models.ApplicationSource,
            sourceHeader?: JSX.Element
        ) => {
            const showChartNonMetadataInfo = (aRevision: string, aRepoUrl: string) => {
                return (
                    <>
                        <div className='row white-box__details-row'>
                            <div className='columns small-3'>Revision:</div>
                            <div className='columns small-9'>{aRevision}</div>
                        </div>
                        <div className='row white-box__details-row'>
                            <div className='columns small-3'>OCI Image:</div>
                            <div className='columns small-9'>{aRepoUrl}</div>
                        </div>
                    </>
                );
            };
            return (
                <DataLoader
                    key={indx}
                    input={application}
                    load={input => services.applications.ociMetadata(input.metadata.name, input.metadata.namespace, aRevision, aSourceIndex, aVersionId)}>
                    {(m: OCIMetadata) => {
                        return m ? (
                            <div className='white-box' style={{marginTop: '1.5em'}}>
                                {sourceHeader && sourceHeader}
                                <div className='white-box__details'>
                                    {showChartNonMetadataInfo(aRevision, aSource.repoURL)}
                                    {m.description && (
                                        <div className='row white-box__details-row'>
                                            <div className='columns small-3'>Description:</div>
                                            <div className='columns small-9'>{m.description}</div>
                                        </div>
                                    )}
                                    {m.authors && m.authors.length > 0 && (
                                        <div className='row white-box__details-row'>
                                            <div className='columns small-3'>Maintainers:</div>
                                            <div className='columns small-9'>{m.authors}</div>
                                        </div>
                                    )}
                                </div>
                            </div>
                        ) : (
                            <div key={indx} className='white-box' style={{marginTop: '1.5em'}}>
                                <div>Source {indx + 1}</div>
                                <div className='white-box__details'>
                                    {showChartNonMetadataInfo(aRevision, aSource.repoURL)}
                                    <div className='row white-box__details-row'>
                                        <div className='columns small-3'>Helm Chart:</div>
                                        <div className='columns small-9'>
                                            {aSource.chart}&nbsp;
                                            {
                                                <a
                                                    title={sources[indx].chart}
                                                    onClick={e => {
                                                        e.stopPropagation();
                                                        window.open(aSource.repoURL);
                                                    }}>
                                                    <i className='fa fa-external-link-alt' />
                                                </a>
                                            }
                                        </div>
                                    </div>
                                </div>
                            </div>
                        );
                    }}
                </DataLoader>
            );
        };

        const getContentForChart = (
            aRevision: string,
            aSourceIndex: number | null,
            aVersionId: number | null,
            indx: number,
            aSource: models.ApplicationSource,
            sourceHeader?: JSX.Element
        ) => {
            const showChartNonMetadataInfo = (aRevision: string, aRepoUrl: string) => {
                return (
                    <>
                        <div className='row white-box__details-row'>
                            <div className='columns small-3'>Revision:</div>
                            <div className='columns small-9'>{aRevision}</div>
                        </div>
                        <div className='row white-box__details-row'>
                            <div className='columns small-3'>Chart Source:</div>
                            <div className='columns small-9'>{aRepoUrl}</div>
                        </div>
                    </>
                );
            };
            return (
                <DataLoader
                    key={indx}
                    input={application}
                    load={input => services.applications.revisionChartDetails(input.metadata.name, input.metadata.namespace, aRevision, aSourceIndex, aVersionId)}>
                    {(m: ChartDetails) => {
                        return m ? (
                            <div className='white-box' style={{marginTop: '1.5em'}}>
                                {sourceHeader && sourceHeader}
                                <div className='white-box__details'>
                                    {showChartNonMetadataInfo(aRevision, aSource.repoURL)}
                                    <div className='row white-box__details-row'>
                                        <div className='columns small-3'>Helm Chart:</div>
                                        <div className='columns small-9'>
                                            {aSource.chart}&nbsp;
                                            {m.home && (
                                                <a
                                                    title={m.home}
                                                    onClick={e => {
                                                        e.stopPropagation();
                                                        window.open(m.home);
                                                    }}>
                                                    <i className='fa fa-external-link-alt' />
                                                </a>
                                            )}
                                        </div>
                                    </div>
                                    {m.description && (
                                        <div className='row white-box__details-row'>
                                            <div className='columns small-3'>Description:</div>
                                            <div className='columns small-9'>{m.description}</div>
                                        </div>
                                    )}
                                    {m.maintainers && m.maintainers.length > 0 && (
                                        <div className='row white-box__details-row'>
                                            <div className='columns small-3'>Maintainers:</div>
                                            <div className='columns small-9'>{m.maintainers.join(', ')}</div>
                                        </div>
                                    )}
                                </div>
                            </div>
                        ) : (
                            <div key={indx} className='white-box' style={{marginTop: '1.5em'}}>
                                <div>Source {indx + 1}</div>
                                <div className='white-box__details'>
                                    {showChartNonMetadataInfo(aRevision, aSource.repoURL)}
                                    <div className='row white-box__details-row'>
                                        <div className='columns small-3'>Helm Chart:</div>
                                        <div className='columns small-9'>
                                            {aSource.chart}&nbsp;
                                            {
                                                <a
                                                    title={sources[indx].chart}
                                                    onClick={e => {
                                                        e.stopPropagation();
                                                        window.open(aSource.repoURL);
                                                    }}>
                                                    <i className='fa fa-external-link-alt' />
                                                </a>
                                            }
                                        </div>
                                    </div>
                                </div>
                            </div>
                        );
                    }}
                </DataLoader>
            );
        };

        const getContentForNonChart = (
            aRevision: string,
            aSourceIndex: number,
            aVersionId: number | null,
            indx: number,
            aSource: models.ApplicationSource,
            sourceHeader?: JSX.Element
        ) => {
            const showNonMetadataInfo = (aSource: models.ApplicationSource, aRevision: string) => {
                return (
                    <>
                        <div className='white-box__details'>
                            <div className='row white-box__details-row'>
                                <div className='columns small-3'>SHA:</div>
                                <div className='columns small-9'>
                                    <Revision repoUrl={aSource.repoURL} revision={aRevision} />
                                </div>
                            </div>
                        </div>
                        <div className='white-box__details'>
                            <div className='row white-box__details-row'>
                                <div className='columns small-3'>Source:</div>
                                <div className='columns small-9'>{aSource.repoURL}</div>
                            </div>
                        </div>
                    </>
                );
            };
            return (
                <DataLoader
                    key={indx}
                    load={() => services.applications.revisionMetadata(application.metadata.name, application.metadata.namespace, aRevision, aSourceIndex, aVersionId)}>
                    {metadata =>
                        metadata ? (
                            <div key={indx} className='white-box' style={{marginTop: '1.5em'}}>
                                {sourceHeader && sourceHeader}
                                {showNonMetadataInfo(aSource, aRevision)}
                                <div className='white-box__details'>
                                    <div className='row white-box__details-row'>
                                        <div className='columns small-3'>Date:</div>
                                        <div className='columns small-9'>
                                            <Timestamp date={metadata.date} />
                                        </div>
                                    </div>
                                </div>
                                <div className='white-box__details'>
                                    <div className='row white-box__details-row'>
                                        <div className='columns small-3'>Tags:</div>
                                        <div className='columns small-9'>{((metadata.tags || []).length > 0 && metadata.tags.join(', ')) || 'No tags'}</div>
                                    </div>
                                </div>
                                <div className='white-box__details'>
                                    <div className='row white-box__details-row'>
                                        <div className='columns small-3'>Author:</div>
                                        <div className='columns small-9'>{metadata.author}</div>
                                    </div>
                                </div>
                                <div className='white-box__details'>
                                    <div className='row white-box__details-row'>
                                        <div className='columns small-3'>Message:</div>
                                        <div className='columns small-9' style={{display: 'flex', alignItems: 'center'}}>
                                            <div className='application-details__commit-message'>{renderCommitMessage(metadata.message)}</div>
                                        </div>
                                    </div>
                                </div>
                            </div>
                        ) : (
                            <div key={indx} className='white-box' style={{marginTop: '1.5em'}}>
                                <div>Source {indx + 1}</div>
                                {showNonMetadataInfo(aSource, aRevision)}
                            </div>
                        )
                    }
                </DataLoader>
            );
        };
        const cont: JSX.Element[] = [];
        const sources: models.ApplicationSource[] = application.spec.sources;
        if (sources?.length > 0 && revisions) {
            revisions.forEach((rev, indx) => {
                if (sources[indx].repoURL.startsWith('oci://')) {
                    cont.push(getContentForOci(rev, indx, getAppCurrentVersion(application), indx, sources[indx], <div>Source {indx + 1}</div>));
                } else if (sources[indx].chart) {
                    cont.push(getContentForChart(rev, indx, getAppCurrentVersion(application), indx, sources[indx], <div>Source {indx + 1}</div>));
                } else {
                    cont.push(getContentForNonChart(rev, indx, getAppCurrentVersion(application), indx, sources[indx], <div>Source {indx + 1}</div>));
                }
            });
            return <>{cont}</>;
        } else if (application.spec.source) {
            if (source.repoURL.startsWith('oci://')) {
                cont.push(getContentForOci(revision, null, getAppCurrentVersion(application), 0, source));
            } else if (source.chart) {
                cont.push(getContentForChart(revision, null, null, 0, source));
            } else {
                cont.push(getContentForNonChart(revision, null, getAppCurrentVersion(application), 0, source));
            }
            return <>{cont}</>;
        } else {
            return (
                <div className='white-box' style={{marginTop: '1.5em'}}>
                    <div className='white-box__details'>
                        <div className='row white-box__details-row'>
                            <div className='columns small-9'>No other information available</div>
                        </div>
                    </div>
                </div>
            );
        }
    }, []);

    const render = () => {
        return (
            <ObservableQuery>
                {q => (
                    <DataLoader
                        errorRenderer={error => <Page title='Application Details'>{error}</Page>}
                        loadingRenderer={() => <Page title='Application Details'>Loading...</Page>}
                        input={props.match.params.name}
                        load={name =>
                            combineLatest([loadAppInfo(name, getAppNamespace()), services.viewPreferences.getPreferences(), q]).pipe(
                                map(items => {
                                    const application = items[0].application;
                                    const pref = items[1].appDetails;
                                    const params = items[2];
                                    if (params.get('resource') != null) {
                                        pref.resourceFilter = params
                                            .get('resource')
                                            .split(',')
                                            .filter(item => !!item);
                                    }
                                    if (params.get('view') != null) {
                                        pref.view = params.get('view') as AppsDetailsViewType;
                                    } else {
                                        const appDefaultView = (application.metadata &&
                                            application.metadata.annotations &&
                                            application.metadata.annotations[appModels.AnnotationDefaultView]) as AppsDetailsViewType;
                                        if (appDefaultView != null) {
                                            pref.view = appDefaultView;
                                        }
                                    }
                                    if (params.get('orphaned') != null) {
                                        pref.orphanedResources = params.get('orphaned') === 'true';
                                    }
                                    if (params.get('podSortMode') != null) {
                                        pref.podView.sortMode = params.get('podSortMode') as PodGroupType;
                                    } else {
                                        const appDefaultPodSort = (application.metadata &&
                                            application.metadata.annotations &&
                                            application.metadata.annotations[appModels.AnnotationDefaultPodSort]) as PodGroupType;
                                        if (appDefaultPodSort != null) {
                                            pref.podView.sortMode = appDefaultPodSort;
                                        }
                                    }
                                    return {...items[0], pref};
                                })
                            )
                        }>
                        {({application, tree, pref}: {application: appModels.Application; tree: appModels.ApplicationTree; pref: AppDetailsPreferences}) => {
                            tree.nodes = tree.nodes || [];
                            const treeFilter = getTreeFilter(pref.resourceFilter);
                            const setFilter = (items: string[]) => {
                                appContext.navigation.goto('.', {resource: items.join(',')}, {replace: true});
                                services.viewPreferences.updatePreferences({appDetails: {...pref, resourceFilter: items}});
                            };
                            const clearFilter = () => setFilter([]);
                            const refreshing = application.metadata.annotations && application.metadata.annotations[appModels.AnnotationRefreshKey];
                            const appNodesByName = groupAppNodesByKey(application, tree);
                            const selectedItem = (selectedNodeKey && appNodesByName.get(selectedNodeKey)) || null;
                            const isAppSelected = selectedItem === application;
                            const selectedNode = !isAppSelected && (selectedItem as appModels.ResourceNode);
                            const operationState = application.status.operationState;
                            const hydrateOperationState = application.status.sourceHydrator.currentOperation;
                            const conditions = application.status.conditions || [];
                            const syncResourceKey = new URLSearchParams(props.history.location.search).get('deploy');
                            const tab = new URLSearchParams(props.history.location.search).get('tab');
                            const source = getAppDefaultSource(application);
                            const showToolTip = pref?.userHelpTipMsgs.find(usrMsg => usrMsg.appName === application.metadata.name);
                            const resourceNodes = (): any[] => {
                                const statusByKey = new Map<string, models.ResourceStatus>();
                                application.status.resources.forEach(res => statusByKey.set(AppUtils.nodeKey(res), res));
                                const resources = new Map<string, any>();
                                tree.nodes
                                    .map(node => ({...node, orphaned: false}))
                                    .concat(((pref.orphanedResources && tree.orphanedNodes) || []).map(node => ({...node, orphaned: true})))
                                    .forEach(node => {
                                        const resource: any = {...node};
                                        resource.uid = node.uid;
                                        const status = statusByKey.get(AppUtils.nodeKey(node));
                                        if (status) {
                                            resource.health = status.health;
                                            resource.status = status.status;
                                            resource.hook = status.hook;
                                            resource.syncWave = status.syncWave;
                                            resource.requiresPruning = status.requiresPruning;
                                        }
                                        resources.set(node.uid || AppUtils.nodeKey(node), resource);
                                    });
                                const resourcesRef = Array.from(resources.values());
                                return resourcesRef;
                            };

                            const filteredRes = resourceNodes().filter(res => {
                                const resNode: ResourceTreeNode = {...res, root: null, info: null, parentRefs: [], resourceVersion: '', uid: ''};
                                resNode.root = resNode;
                                return filterTreeNode(resNode, treeFilter);
                            });
                            const openGroupNodeDetails = (groupdedNodeIds: string[]) => {
                                const resources = resourceNodes();
                                setState(prevState => ({
                                    ...prevState,
                                    groupedResources: groupdedNodeIds
                                        ? resources.filter(res => groupdedNodeIds.includes(res.uid) || groupdedNodeIds.includes(AppUtils.nodeKey(res)))
                                        : []
                                }));
                            };
                            const {Tree, Pods, Network, List} = AppsDetailsViewKey;
                            const zoomNum = (pref.zoom * 100).toFixed(0);
                            const setZoom = (s: number) => {
                                let targetZoom: number = pref.zoom + s;
                                if (targetZoom <= 0.05) {
                                    targetZoom = 0.1;
                                } else if (targetZoom > 2.0) {
                                    targetZoom = 2.0;
                                }
                                services.viewPreferences.updatePreferences({appDetails: {...pref, zoom: targetZoom}});
                            };
                            const setFilterGraph = (filterGraph: any[]) => {
                                setState(prevState => ({...prevState, filteredGraph: filterGraph}));
                            };
                            const setShowCompactNodes = (showCompactView: boolean) => {
                                services.viewPreferences.updatePreferences({appDetails: {...pref, groupNodes: showCompactView}});
                            };
                            const updateHelpTipState = (usrHelpTip: models.UserMessages) => {
                                const existingIndex = pref.userHelpTipMsgs.findIndex(msg => msg.appName === usrHelpTip.appName && msg.msgKey === usrHelpTip.msgKey);
                                if (existingIndex !== -1) {
                                    pref.userHelpTipMsgs[existingIndex] = usrHelpTip;
                                } else {
                                    (pref.userHelpTipMsgs || []).push(usrHelpTip);
                                }
                            };
                            const toggleNodeName = () => {
                                setState(prevState => ({...prevState, showFullNodeName: !prevState.showFullNodeName}));
                            };
                            const toggleNameDirection = () => {
                                setState(prevState => ({...prevState, truncateNameOnRight: !prevState.truncateNameOnRight}));
                            };
                            const expandAll = () => {
                                setState(prevState => ({...prevState, collapsedNodes: []}));
                            };
                            const collapseAll = () => {
                                const nodes = new Array<ResourceTreeNode>();
                                tree.nodes
                                    .map(node => ({...node, orphaned: false}))
                                    .concat((tree.orphanedNodes || []).map(node => ({...node, orphaned: true})))
                                    .forEach(node => {
                                        const resourceNode: ResourceTreeNode = {...node};
                                        nodes.push(resourceNode);
                                    });
                                const collapsedNodesList = state.collapsedNodes.slice();
                                if (pref.view === 'network') {
                                    const networkNodes = nodes.filter(node => node.networkingInfo);
                                    networkNodes.forEach(parent => {
                                        const parentId = parent.uid;
                                        if (collapsedNodesList.indexOf(parentId) < 0) {
                                            collapsedNodesList.push(parentId);
                                        }
                                    });
                                    setState(prevState => ({...prevState, collapsedNodes: collapsedNodesList}));
                                } else {
                                    const managedKeys = new Set(application.status.resources.map(AppUtils.nodeKey));
                                    nodes.forEach(node => {
                                        if (!((node.parentRefs || []).length === 0 || managedKeys.has(AppUtils.nodeKey(node)))) {
                                            node.parentRefs.forEach(parent => {
                                                const parentId = parent.uid;
                                                if (collapsedNodesList.indexOf(parentId) < 0) {
                                                    collapsedNodesList.push(parentId);
                                                }
                                            });
                                        }
                                    });
                                    collapsedNodesList.push(application.kind + '-' + application.metadata.namespace + '-' + application.metadata.name);
                                    setState(prevState => ({...prevState, collapsedNodes: collapsedNodesList}));
                                }
                            };
                            const appFullName = AppUtils.nodeKey({
                                group: 'argoproj.io',
                                kind: application.kind,
                                name: application.metadata.name,
                                namespace: application.metadata.namespace
                            });

                            const activeStatusExt = state.statusExtensionsMap[selectedExtension];
                            const activeTopBarActionMenuExt = state.topBarActionMenuExtsMap[selectedExtension];

                            if (state.extensionsMap[pref.view] != null) {
                                const extension = state.extensionsMap[pref.view];
                                if (!extension.shouldDisplay(application)) {
                                    appContext.navigation.goto('.', {view: Tree});
                                }
                            }

                            return (
                                <div className={`application-details ${props.match.params.name}`}>
                                    <Page
                                        title={props.match.params.name + ' - ' + getPageTitle(pref.view)}
                                        useTitleOnly={true}
                                        topBarTitle={getPageTitle(pref.view)}
                                        toolbar={{
                                            breadcrumbs: [
                                                {title: 'Applications', path: '/applications'},
                                                {title: <ApplicationsDetailsAppDropdown appName={props.match.params.name} objectListKind={objectListKind} />}
                                            ],
                                            actionMenu: {
                                                items: [
                                                    ...getApplicationActionMenu(application, true),
                                                    ...(state.topBarActionMenuExts
                                                        ?.filter(ext => ext.shouldDisplay?.(application))
                                                        .map(ext => renderActionMenuItem(ext, tree, application, setExtensionPanelVisible)) || [])
                                                ]
                                            },
                                            tools: (
                                                <React.Fragment key='app-list-tools'>
                                                    <div className='application-details__view-type'>
                                                        <i
                                                            className={classNames('fa fa-sitemap', {selected: pref.view === Tree})}
                                                            title='Tree'
                                                            onClick={() => {
                                                                appContext.navigation.goto('.', {view: Tree});
                                                                services.viewPreferences.updatePreferences({appDetails: {...pref, view: Tree}});
                                                            }}
                                                        />
                                                        <i
                                                            className={classNames('fa fa-th', {selected: pref.view === Pods})}
                                                            title='Pods'
                                                            onClick={() => {
                                                                appContext.navigation.goto('.', {view: Pods});
                                                                services.viewPreferences.updatePreferences({appDetails: {...pref, view: Pods}});
                                                            }}
                                                        />
                                                        <i
                                                            className={classNames('fa fa-network-wired', {selected: pref.view === Network})}
                                                            title='Network'
                                                            onClick={() => {
                                                                appContext.navigation.goto('.', {view: Network});
                                                                services.viewPreferences.updatePreferences({appDetails: {...pref, view: Network}});
                                                            }}
                                                        />
                                                        <i
                                                            className={classNames('fa fa-th-list', {selected: pref.view === List})}
                                                            title='List'
                                                            onClick={() => {
                                                                appContext.navigation.goto('.', {view: List});
                                                                services.viewPreferences.updatePreferences({appDetails: {...pref, view: List}});
                                                            }}
                                                        />
                                                        {state.extensions &&
                                                            (state.extensions || [])
                                                                .filter(ext => ext.shouldDisplay(application))
                                                                .map(ext => (
                                                                    <i
                                                                        key={ext.title}
                                                                        className={classNames(`fa ${ext.icon}`, {selected: pref.view === ext.title})}
                                                                        title={ext.title}
                                                                        onClick={() => {
                                                                            appContext.navigation.goto('.', {view: ext.title});
                                                                            services.viewPreferences.updatePreferences({appDetails: {...pref, view: ext.title}});
                                                                        }}
                                                                    />
                                                                ))}
                                                    </div>
                                                </React.Fragment>
                                            )
                                        }}>
                                        <div className='application-details__wrapper'>
                                            <div className='application-details__status-panel'>
                                                <ApplicationStatusPanel
                                                    application={application}
                                                    showDiff={() => selectNode(appFullName, 0, 'diff')}
                                                    showOperation={() => setOperationStatusVisible(true)}
                                                    showHydrateOperation={() => setHydrateOperationStatusVisible(true)}
                                                    showConditions={() => setConditionsStatusVisible(true)}
                                                    showExtension={id => setExtensionPanelVisible(id)}
                                                    showMetadataInfo={revision => setState(prevState => ({...prevState, revision}))}
                                                />
                                            </div>
                                            <div className='application-details__tree'>
                                                {refreshing && <p className='application-details__refreshing-label'>Refreshing</p>}
                                                {((pref.view === 'tree' || pref.view === 'network') && (
                                                    <>
                                                        <DataLoader load={() => services.viewPreferences.getPreferences()}>
                                                            {viewPref => (
                                                                <ApplicationDetailsFilters
                                                                    pref={pref}
                                                                    tree={tree}
                                                                    onSetFilter={setFilter}
                                                                    onClearFilter={clearFilter}
                                                                    collapsed={viewPref.hideSidebar}
                                                                    resourceNodes={state.filteredGraph}
                                                                />
                                                            )}
                                                        </DataLoader>
                                                        <div className='graph-options-panel'>
                                                            <a
                                                                className={`group-nodes-button`}
                                                                onClick={() => {
                                                                    toggleNameDirection();
                                                                }}
                                                                title={state.truncateNameOnRight ? 'Truncate resource name right' : 'Truncate resource name left'}>
                                                                <i
                                                                    className={classNames({
                                                                        'fa fa-align-right': state.truncateNameOnRight,
                                                                        'fa fa-align-left': !state.truncateNameOnRight
                                                                    })}
                                                                />
                                                            </a>
                                                            <a
                                                                className={`group-nodes-button`}
                                                                onClick={() => {
                                                                    toggleNodeName();
                                                                }}
                                                                title={state.showFullNodeName ? 'Show wrapped resource name' : 'Show full resource name'}>
                                                                <i
                                                                    className={classNames({
                                                                        'fa fa-expand': state.showFullNodeName,
                                                                        'fa fa-compress': !state.showFullNodeName
                                                                    })}
                                                                />
                                                            </a>
                                                            {(pref.view === 'tree' || pref.view === 'network') && (
                                                                <Tooltip
                                                                    content={AppUtils.userMsgsList[showToolTip?.msgKey] || 'Group Nodes'}
                                                                    visible={pref.groupNodes && showToolTip !== undefined && !showToolTip?.display}
                                                                    duration={showToolTip?.duration}
                                                                    zIndex={1}>
                                                                    <a
                                                                        className={`group-nodes-button group-nodes-button${!pref.groupNodes ? '' : '-on'}`}
                                                                        title={pref.view === 'tree' ? 'Group Nodes' : 'Collapse Pods'}
                                                                        onClick={() => toggleCompactView(application.metadata.name, pref)}>
                                                                        <i className={classNames('fa fa-object-group fa-fw')} />
                                                                    </a>
                                                                </Tooltip>
                                                            )}
                                                            <span className={`separator`} />
                                                            <a className={`group-nodes-button`} onClick={() => expandAll()} title='Expand all child nodes of all parent nodes'>
                                                                <i className='fa fa-plus fa-fw' />
                                                            </a>
                                                            <a className={`group-nodes-button`} onClick={() => collapseAll()} title='Collapse all child nodes of all parent nodes'>
                                                                <i className='fa fa-minus fa-fw' />
                                                            </a>
                                                            <span className={`separator`} />
                                                            <span>
                                                                <a className={`group-nodes-button`} onClick={() => setZoom(0.1)} title='Zoom in'>
                                                                    <i className='fa fa-search-plus fa-fw' />
                                                                </a>
                                                                <a className={`group-nodes-button`} onClick={() => setZoom(-0.1)} title='Zoom out'>
                                                                    <i className='fa fa-search-minus fa-fw' />
                                                                </a>
                                                                <div className={`zoom-value`}>{zoomNum}%</div>
                                                            </span>
                                                        </div>
                                                        <ApplicationResourceTree
                                                            nodeFilter={node => filterTreeNode(node, treeFilter)}
                                                            selectedNodeFullName={selectedNodeKey}
                                                            onNodeClick={fullName => selectNode(fullName)}
                                                            nodeMenu={node =>
                                                                AppUtils.renderResourceMenu(node, application, tree, appContext, appChanged.current, () =>
                                                                    getApplicationActionMenu(application, false)
                                                                )
                                                            }
                                                            showCompactNodes={pref.groupNodes}
                                                            userMsgs={pref.userHelpTipMsgs}
                                                            tree={tree}
                                                            app={application}
                                                            showOrphanedResources={pref.orphanedResources}
                                                            useNetworkingHierarchy={pref.view === 'network'}
                                                            onClearFilter={clearFilter}
                                                            onGroupdNodeClick={groupdedNodeIds => openGroupNodeDetails(groupdedNodeIds)}
                                                            zoom={pref.zoom}
                                                            podGroupCount={pref.podGroupCount}
                                                            appContext={{...appContext, apis: appContext} as unknown as AppContext}
                                                            nameDirection={state.truncateNameOnRight}
                                                            nameWrap={state.showFullNodeName}
                                                            filters={pref.resourceFilter}
                                                            setTreeFilterGraph={setFilterGraph}
                                                            updateUsrHelpTipMsgs={updateHelpTipState}
                                                            setShowCompactNodes={setShowCompactNodes}
                                                            setNodeExpansion={(node, isExpanded) => setNodeExpansion(node, isExpanded)}
                                                            getNodeExpansion={node => getNodeExpansion(node)}
                                                        />
                                                    </>
                                                )) ||
                                                    (pref.view === 'pods' && (
                                                        <PodView
                                                            tree={tree}
                                                            app={application}
                                                            onItemClick={fullName => selectNode(fullName)}
                                                            nodeMenu={node =>
                                                                AppUtils.renderResourceMenu(node, application, tree, appContext, appChanged.current, () =>
                                                                    getApplicationActionMenu(application, false)
                                                                )
                                                            }
                                                            quickStarts={node => AppUtils.renderResourceButtons(node, application, tree, appContext, appChanged.current)}
                                                        />
                                                    )) ||
                                                    (state.extensionsMap[pref.view] != null && (
                                                        <ExtensionView extension={state.extensionsMap[pref.view]} application={application} tree={tree} />
                                                    )) || (
                                                        <div>
                                                            <DataLoader load={() => services.viewPreferences.getPreferences()}>
                                                                {viewPref => (
                                                                    <ApplicationDetailsFilters
                                                                        pref={pref}
                                                                        tree={tree}
                                                                        onSetFilter={setFilter}
                                                                        onClearFilter={clearFilter}
                                                                        collapsed={viewPref.hideSidebar}
                                                                        resourceNodes={filteredRes}
                                                                    />
                                                                )}
                                                            </DataLoader>
                                                            {(filteredRes.length > 0 && (
                                                                <Paginate
                                                                    page={state.page}
                                                                    data={filteredRes}
                                                                    onPageChange={page => setState(prevState => ({...prevState, page}))}
                                                                    preferencesKey='application-details'>
                                                                    {data => (
                                                                        <ApplicationResourceList
                                                                            pref={pref}
                                                                            onNodeClick={fullName => selectNode(fullName)}
                                                                            resources={data}
                                                                            nodeMenu={node =>
                                                                                AppUtils.renderResourceMenu(node, application, tree, appContext, appChanged.current, () =>
                                                                                    getApplicationActionMenu(application, false)
                                                                                )
                                                                            }
                                                                            tree={tree}
                                                                        />
                                                                    )}
                                                                </Paginate>
                                                            )) || (
                                                                <EmptyState icon='fa fa-search'>
                                                                    <h4>No resources found</h4>
                                                                    <h5>Try to change filter criteria</h5>
                                                                </EmptyState>
                                                            )}
                                                        </div>
                                                    )}
                                            </div>
                                        </div>
                                        <SlidingPanel isShown={state.groupedResources.length > 0} onClose={() => closeGroupedNodesPanel()}>
                                            <div className='application-details__sliding-panel-pagination-wrap'>
                                                <Paginate
                                                    page={state.slidingPanelPage}
                                                    data={state.groupedResources}
                                                    onPageChange={page => setState(prevState => ({...prevState, slidingPanelPage: page}))}
                                                    preferencesKey='grouped-nodes-details'>
                                                    {data => (
                                                        <ApplicationResourceList
                                                            pref={pref}
                                                            onNodeClick={fullName => selectNode(fullName)}
                                                            resources={data}
                                                            nodeMenu={node =>
                                                                AppUtils.renderResourceMenu(node, application, tree, appContext, appChanged.current, () =>
                                                                    getApplicationActionMenu(application, false)
                                                                )
                                                            }
                                                            tree={tree}
                                                        />
                                                    )}
                                                </Paginate>
                                            </div>
                                        </SlidingPanel>
                                        <SlidingPanel isShown={selectedNode != null || isAppSelected} onClose={() => selectNode('')}>
                                            <ResourceDetails
                                                tree={tree}
                                                application={application}
                                                isAppSelected={isAppSelected}
                                                updateApp={(app: models.Application, query: {validate?: boolean}) => updateApp(app, query)}
                                                selectedNode={selectedNode}
                                                appCxt={{...appContext, apis: appContext} as unknown as AppContext}
                                                tab={tab}
                                            />
                                        </SlidingPanel>
                                        <ApplicationSyncPanel
                                            application={application}
                                            hide={() => AppUtils.showDeploy(null, null, appContext)}
                                            selectedResource={syncResourceKey}
                                        />
                                        <SlidingPanel isShown={selectedRollbackDeploymentIndex > -1} onClose={() => setRollbackPanelVisible(-1)}>
                                            {selectedRollbackDeploymentIndex > -1 && (
                                                <ApplicationDeploymentHistory
                                                    app={application}
                                                    rollbackApp={info => rollbackApplication(info, application)}
                                                    selectDeployment={i => setRollbackPanelVisible(i)}
                                                />
                                            )}
                                        </SlidingPanel>
                                        <SlidingPanel isShown={showOperationState && !!operationState} onClose={() => setOperationStatusVisible(false)}>
                                            {operationState && <ApplicationOperationState application={application} operationState={operationState} />}
                                        </SlidingPanel>
                                        <SlidingPanel isShown={showHydrateOperationState && !!hydrateOperationState} onClose={() => setHydrateOperationStatusVisible(false)}>
                                            {hydrateOperationState && <ApplicationHydrateOperationState hydrateOperationState={hydrateOperationState} />}
                                        </SlidingPanel>
                                        <SlidingPanel isShown={showConditions && !!conditions} onClose={() => setConditionsStatusVisible(false)}>
                                            {conditions && <ApplicationConditions conditions={conditions} />}
                                        </SlidingPanel>
                                        <SlidingPanel
                                            isShown={state.revision === 'SYNC_STATUS_REVISION' || state.revision === 'OPERATION_STATE_REVISION'}
                                            isMiddle={true}
                                            onClose={() => setState(prevState => ({...prevState, revision: null}))}>
                                            {state.revision === 'SYNC_STATUS_REVISION' &&
                                                (application.status.sync.revisions || application.status.sync.revision) &&
                                                getContent(application, source, application.status.sync.revisions, application.status.sync.revision)}
                                            {state.revision === 'OPERATION_STATE_REVISION' &&
                                                (application.status.operationState.syncResult.revisions || application.status.operationState.syncResult.revision) &&
                                                getContent(
                                                    application,
                                                    source,
                                                    application.status.operationState.syncResult.revisions,
                                                    application.status.operationState.syncResult.revision
                                                )}
                                        </SlidingPanel>
                                        <SlidingPanel
                                            isShown={selectedExtension !== '' && activeStatusExt != null && activeStatusExt.flyout != null}
                                            onClose={() => setExtensionPanelVisible('')}>
                                            {selectedExtension !== '' && activeStatusExt?.flyout && <activeStatusExt.flyout application={application} tree={tree} />}
                                        </SlidingPanel>
                                        <SlidingPanel
                                            isMiddle={activeTopBarActionMenuExt?.isMiddle ?? true}
                                            isShown={selectedExtension !== '' && activeTopBarActionMenuExt != null && activeTopBarActionMenuExt.flyout != null}
                                            onClose={() => setExtensionPanelVisible('')}>
                                            {selectedExtension !== '' && activeTopBarActionMenuExt?.flyout && (
                                                <activeTopBarActionMenuExt.flyout application={application} tree={tree} />
                                            )}
                                        </SlidingPanel>
                                    </Page>
                                </div>
                            );
                        }}
                    </DataLoader>
                )}
            </ObservableQuery>
        );
    };

    const renderActionMenuItem = useCallback(
        (ext: TopBarActionMenuExt, tree: appModels.ApplicationTree, application: appModels.Application, showExtension?: (id: string) => any): any => {
            return {
                action: () => setExtensionPanelVisible(ext.id),
                title: <ext.component application={application} tree={tree} openFlyout={() => showExtension && showExtension(ext.id)} />,
                iconClassName: ext.iconClassName
            };
        },
        [setExtensionPanelVisible]
    );

    const getApplicationActionMenu = useCallback(
        (app: appModels.Application, needOverlapLabelOnNarrowScreen: boolean) => {
            const refreshing = app.metadata.annotations && app.metadata.annotations[appModels.AnnotationRefreshKey];
            const fullName = AppUtils.nodeKey({group: 'argoproj.io', kind: app.kind, name: app.metadata.name, namespace: app.metadata.namespace});
            const ActionMenuItem = (prop: {actionLabel: string}) => <span className={needOverlapLabelOnNarrowScreen ? 'show-for-large' : ''}>{prop.actionLabel}</span>;
            return [
                {
                    iconClassName: 'fa fa-info-circle',
                    title: <ActionMenuItem actionLabel='Details' />,
                    action: () => selectNode(fullName),
                    disabled: !app.spec.source && (!app.spec.sources || app.spec.sources.length === 0) && !app.spec.sourceHydrator
                },
                {
                    iconClassName: 'fa fa-file-medical',
                    title: <ActionMenuItem actionLabel='Diff' />,
                    action: () => selectNode(fullName, 0, 'diff'),
                    disabled:
                        app.status.sync.status === appModels.SyncStatuses.Synced ||
                        (!app.spec.source && (!app.spec.sources || app.spec.sources.length === 0) && !app.spec.sourceHydrator)
                },
                {
                    iconClassName: 'fa fa-sync',
                    title: <ActionMenuItem actionLabel='Sync' />,
                    action: () => AppUtils.showDeploy('all', null, appContext),
                    disabled: !app.spec.source && (!app.spec.sources || app.spec.sources.length === 0) && !app.spec.sourceHydrator
                },
                ...(app.status?.operationState?.phase === 'Running' && app.status.resources.find(r => r.requiresDeletionConfirmation)
                    ? [
                          {
                              iconClassName: 'fa fa-check',
                              title: <ActionMenuItem actionLabel='Confirm Pruning' />,
                              action: () => confirmDeletion(app, 'Confirm Prunning', 'Are you sure you want to confirm resources pruning?')
                          }
                      ]
                    : []),
                {
                    iconClassName: 'fa fa-info-circle',
                    title: <ActionMenuItem actionLabel='Sync Status' />,
                    action: () => setOperationStatusVisible(true),
                    disabled: !app.status.operationState
                },
                {
                    iconClassName: 'fa fa-history',
                    title: <ActionMenuItem actionLabel='History and rollback' />,
                    action: () => {
                        setRollbackPanelVisible(0);
                    },
                    disabled: !app.status.operationState
                },
                app.metadata.deletionTimestamp &&
                app.status.resources.find(r => r.requiresDeletionConfirmation) &&
                !((app.metadata.annotations || {})[appModels.AppDeletionConfirmedAnnotation] == 'true')
                    ? {
                          iconClassName: 'fa fa-check',
                          title: <ActionMenuItem actionLabel='Confirm Deletion' />,
                          action: () => confirmDeletion(app, 'Confirm Deletion', 'Are you sure you want to delete this application?')
                      }
                    : {
                          iconClassName: 'fa fa-times-circle',
                          title: <ActionMenuItem actionLabel='Delete' />,
                          action: () => deleteApplication(),
                          disabled: !!app.metadata.deletionTimestamp
                      },
                {
                    iconClassName: classNames('fa fa-redo', {'status-icon--spin': !!refreshing}),
                    title: (
                        <React.Fragment>
                            <ActionMenuItem actionLabel='Refresh' />{' '}
                            <DropDownMenu
                                items={[
                                    {
                                        title: 'Hard Refresh',
                                        action: () => !refreshing && services.applications.get(app.metadata.name, app.metadata.namespace, objectListKind, 'hard')
                                    }
                                ]}
                                anchor={() => (
                                    <button className='argo-button--base application-details__dropdown-anchor-inner'>
                                        <i className='fa fa-caret-down' />
                                    </button>
                                )}
                            />
                        </React.Fragment>
                    ),
                    disabled: !!refreshing,
                    action: () => {
                        if (!refreshing) {
                            services.applications.get(app.metadata.name, app.metadata.namespace, objectListKind, 'normal');
                            AppUtils.setAppRefreshing(app);
                            appChanged.current.next(app);
                        }
                    }
                }
            ];
        },
        [selectNode, appContext, confirmDeletion, setOperationStatusVisible, setRollbackPanelVisible, deleteApplication, objectListKind]
    );

    const filterTreeNode = useCallback(
        (node: ResourceTreeNode, filterInput: FilterInput): boolean => {
            const syncStatuses = filterInput.sync.map(item => (item === 'OutOfSync' ? ['OutOfSync', 'Unknown'] : [item])).reduce((first, second) => first.concat(second), []);

            const root = node.root || ({} as ResourceTreeNode);
            const hook = root && root.hook;
            if (
                (filterInput.name.length === 0 || nodeNameMatchesWildcardFilters(node.name, filterInput.name)) &&
                (filterInput.kind.length === 0 || filterInput.kind.indexOf(node.kind) > -1) &&
                // include if node's root sync matches filter
                (syncStatuses.length === 0 || hook || (root.status && syncStatuses.indexOf(root.status) > -1)) &&
                // include if node or node's root health matches filter
                (filterInput.health.length === 0 ||
                    hook ||
                    (root.health && filterInput.health.indexOf(root.health.status) > -1) ||
                    (node.health && filterInput.health.indexOf(node.health.status) > -1)) &&
                (filterInput.namespace.length === 0 || filterInput.namespace.includes(node.namespace))
            ) {
                return true;
            }

            return false;
        },
        [nodeNameMatchesWildcardFilters]
    );

    const onAppDeleted = useCallback(() => {
        appContext.notifications.show({type: NotificationType.Success, content: `Application '${props.match.params.name}' was deleted`});
        appContext.navigation.goto('/applications');
    }, [appContext, props.match.params.name]);

    const loadAppInfo = useCallback(
        (name: string, appNamespace: string): Observable<{application: appModels.AbstractApplication; tree: appModels.AbstractApplicationTree}> => {
            return from(services.applications.get(name, appNamespace, objectListKind))
                .pipe(
                    mergeMap(app => {
                        const fallbackTree = {
                            nodes:
                                app.status?.resources?.map((res: appModels.ResourceStatus) => ({
                                    ...res,
                                    parentRefs: [] as appModels.ResourceRef[],
                                    info: [] as appModels.InfoItem[],
                                    resourceVersion: '',
                                    uid: ''
                                })) || [],
                            orphanedNodes: [] as appModels.ResourceNode[],
                            hosts: [] as appModels.Node[]
                        } as appModels.ApplicationTree;
                        return combineLatest(
                            merge(
                                from([app]),
                                appChanged.current.pipe(filter(item => !!item)),
                                AppUtils.handlePageVisibility(() =>
                                    services.applications
                                        .watch(objectListKind, {name, appNamespace})
                                        .pipe(
                                            map(watchEvent => {
                                                if (watchEvent.type === 'DELETED') {
                                                    onAppDeleted();
                                                }
                                                return watchEvent.application;
                                            })
                                        )
                                        .pipe(repeat())
                                        .pipe(retryWhen(errors => errors.pipe(delay(500))))
                                )
                            ),
                            merge(
                                from([fallbackTree]),
                                services.applications.resourceTree(name, appNamespace, objectListKind).catch(() => fallbackTree),
                                AppUtils.handlePageVisibility(() =>
                                    services.applications
                                        .watchResourceTree(name, appNamespace, objectListKind)
                                        .pipe(repeat())
                                        .pipe(retryWhen(errors => errors.pipe(delay(500))))
                                )
                            )
                        );
                    })
                )
                .pipe(filter(([application, tree]) => !!application && !!tree))
                .pipe(map(([application, tree]) => ({application, tree})));
        },
        [onAppDeleted, objectListKind]
    );

    const updateApp = useCallback(
        async (app: appModels.Application, query: {validate?: boolean}) => {
            const latestApp = await services.applications.get(app.metadata.name, app.metadata.namespace, objectListKind);
            latestApp.metadata.labels = app.metadata.labels;
            latestApp.metadata.annotations = app.metadata.annotations;
            latestApp.spec = app.spec;
            const updatedApp = await services.applications.update(latestApp, query);
            appChanged.current.next(updatedApp);
        },
        [objectListKind]
    );

    const groupAppNodesByKey = useCallback((application: appModels.Application, tree: appModels.ApplicationTree) => {
        const nodeByKey = new Map<string, appModels.ResourceDiff | appModels.ResourceNode | appModels.Application>();
        tree.nodes.concat(tree.orphanedNodes || []).forEach(node => nodeByKey.set(AppUtils.nodeKey(node), node));
        nodeByKey.set(AppUtils.nodeKey({group: 'argoproj.io', kind: application.kind, name: application.metadata.name, namespace: application.metadata.namespace}), application);
        return nodeByKey;
    }, []);

    const getTreeFilter = useCallback((filterInput: string[]): FilterInput => {
        const name = new Array<string>();
        const kind = new Array<string>();
        const health = new Array<string>();
        const sync = new Array<string>();
        const namespace = new Array<string>();
        for (const item of filterInput || []) {
            const [type, val] = item.split(':');
            switch (type) {
                case 'name':
                    name.push(val);
                    break;
                case 'kind':
                    kind.push(val);
                    break;
                case 'health':
                    health.push(val);
                    break;
                case 'sync':
                    sync.push(val);
                    break;
                case 'namespace':
                    namespace.push(val);
                    break;
            }
        }
        return {kind, health, sync, namespace, name};
    }, []);

    // Return the render function
    return render();
};

const ExtensionView = (props: {extension: AppViewExtension; application: models.Application; tree: models.ApplicationTree}) => {
    const {extension, application, tree} = props;
    return <extension.component application={application} tree={tree} />;
};
