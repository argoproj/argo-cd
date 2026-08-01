import {SlidingPanel} from 'argo-ui';
import * as React from 'react';
import {Spinner} from '../../../shared/components';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ApplicationDiffAccordion} from './ApplicationDiffAccordion';

import './global-diff.scss';

export interface GlobalDiffModalProps {
    isShown: boolean;
    onClose: () => void;
    apps?: models.ApplicationIdentifier[];
    projects?: string[];
    selector?: string;
    allApps?: models.Application[];
}

const PAGE_SIZE = 50;

export const GlobalDiffModal = (props: GlobalDiffModalProps) => {
    const {isShown, onClose, apps, projects, selector, allApps} = props;
    const [loading, setLoading] = React.useState(false);
    const [appDiffs, setAppDiffs] = React.useState<models.ApplicationDiffSummary[]>([]);
    const [searchQuery, setSearchQuery] = React.useState('');
    const [syncingAll, setSyncingAll] = React.useState(false);
    const [refreshTrigger, setRefreshTrigger] = React.useState(0);
    const [refreshStrategy, setRefreshStrategy] = React.useState<string | undefined>(undefined);
    const [currentPage, setCurrentPage] = React.useState(1);

    const [lazyData, setLazyData] = React.useState<Record<string, models.ApplicationDiffSummary>>({});
    const [loadingLazy, setLoadingLazy] = React.useState<Record<string, boolean>>({});

    const [expandedApps, setExpandedApps] = React.useState<Record<string, boolean>>({});

    const handleExpandLazy = React.useCallback(
        async (appName: string, appNamespace: string) => {
            const key = `${appNamespace}/${appName}`;
            if (lazyData[key]) {
                return;
            }
            setLoadingLazy(prev => ({...prev, [key]: true}));
            try {
                const result = await services.applications.getBatchApplicationDiff({
                    apps: [{name: appName, appNamespace}]
                });
                if (result && result.length > 0) {
                    setLazyData(prev => ({...prev, [key]: result[0]}));
                }
            } catch (err) {
                console.error(`Failed to load lazy diff for ${key}`, err);
            } finally {
                setLoadingLazy(prev => ({...prev, [key]: false}));
            }
        },
        [lazyData]
    );

    const isAppExpanded = React.useCallback(
        (appName: string, appNamespace: string) => {
            const key = `${appNamespace}/${appName}`;
            return expandedApps[key] !== false;
        },
        [expandedApps]
    );

    const handleToggleApp = React.useCallback(
        (appName: string, appNamespace: string) => {
            const key = `${appNamespace}/${appName}`;
            const nextOpen = !isAppExpanded(appName, appNamespace);
            setExpandedApps(prev => ({
                ...prev,
                [key]: nextOpen
            }));
            if (nextOpen) {
                const app = appDiffs.find(a => a.appName === appName && a.appNamespace === appNamespace);
                if (app && app.isLazy && !lazyData[key]) {
                    handleExpandLazy(appName, appNamespace);
                }
            }
        },
        [isAppExpanded, appDiffs, lazyData, handleExpandLazy]
    );

    const handleExpandAll = React.useCallback(() => {
        const nextExpanded: Record<string, boolean> = {};
        appDiffs.forEach(app => {
            const key = `${app.appNamespace}/${app.appName}`;
            nextExpanded[key] = true;
            if (app.isLazy && !lazyData[key]) {
                handleExpandLazy(app.appName, app.appNamespace);
            }
        });
        setExpandedApps(nextExpanded);
    }, [appDiffs, lazyData, handleExpandLazy]);

    const handleCollapseAll = React.useCallback(() => {
        const nextExpanded: Record<string, boolean> = {};
        appDiffs.forEach(app => {
            const key = `${app.appNamespace}/${app.appName}`;
            nextExpanded[key] = false;
        });
        setExpandedApps(nextExpanded);
    }, [appDiffs]);

    const targetApps = React.useMemo(() => {
        let list = allApps || [];
        if (apps && apps.length > 0) {
            const appKeys = new Set(apps.map(a => `${a.appNamespace}/${a.name}`));
            list = list.filter(app => appKeys.has(`${app.metadata.namespace}/${app.metadata.name}`));
        } else {
            list = list.filter(app => {
                if (projects && projects.length > 0 && !projects.includes(app.spec.project)) {
                    return false;
                }
                return app.status.sync.status === 'OutOfSync';
            });
        }
        return list;
    }, [allApps, apps, projects]);

    const totalPages = Math.ceil(targetApps.length / PAGE_SIZE);
    const pageApps = React.useMemo(() => {
        const start = (currentPage - 1) * PAGE_SIZE;
        return targetApps.slice(start, start + PAGE_SIZE);
    }, [targetApps, currentPage]);

    const appsKey = (apps || []).map(a => `${a.appNamespace}/${a.name}`).join(',');
    const projectsKey = (projects || []).join(',');
    const pageAppsKey = pageApps.map(app => `${app.metadata.name}/${app.metadata.namespace}`).join(',');

    React.useEffect(() => {
        if (isShown) {
            let active = true;
            Promise.resolve().then(() => {
                if (active) {
                    setCurrentPage(1);
                    setLazyData({});
                    setLoadingLazy({});
                }
            });
            return () => {
                active = false;
            };
        }
    }, [isShown, appsKey, projectsKey, selector]);

    React.useEffect(() => {
        if (isShown && pageApps.length > 0) {
            let active = true;
            Promise.resolve().then(() => {
                if (active) {
                    setLoading(true);
                }
            });

            const pageAppIdentifiers = pageApps.map(app => ({
                name: app.metadata.name,
                appNamespace: app.metadata.namespace
            }));
            services.applications
                .getBatchApplicationDiff({
                    apps: pageAppIdentifiers,
                    projects,
                    selector,
                    refresh: refreshStrategy
                })
                .then(data => {
                    if (active) {
                        const dataSize = JSON.stringify(data).length;
                        const isPayloadTooLarge = dataSize > 1024 * 1024;

                        const processedData = data.map(app => {
                            if (isPayloadTooLarge) {
                                const strippedDiffs = app.diffs.map(
                                    d =>
                                        ({
                                            ...d,
                                            liveState: undefined,
                                            targetState: undefined,
                                            predictedLiveState: undefined,
                                            normalizedLiveState: undefined
                                        }) as any
                                );
                                return {
                                    ...app,
                                    diffs: strippedDiffs,
                                    isLazy: true
                                };
                            }
                            return app;
                        });

                        setAppDiffs(processedData);
                        setLoading(false);

                        processedData.forEach(app => {
                            const key = `${app.appNamespace}/${app.appName}`;
                            if (app.isLazy && isAppExpanded(app.appName, app.appNamespace) && !lazyData[key]) {
                                handleExpandLazy(app.appName, app.appNamespace);
                            }
                        });
                    }
                })
                .catch(err => {
                    console.error('Failed to load global diffs', err);
                    if (active) {
                        setLoading(false);
                    }
                });

            return () => {
                active = false;
            };
        } else if (isShown && pageApps.length === 0) {
            let active = true;
            Promise.resolve().then(() => {
                if (active) {
                    setAppDiffs([]);
                }
            });
            return () => {
                active = false;
            };
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [isShown, pageAppsKey, refreshTrigger, refreshStrategy, projectsKey, selector]);

    const handleRefresh = () => {
        setRefreshStrategy('normal');
        setRefreshTrigger(prev => prev + 1);
    };

    const handleSyncAllSelected = async () => {
        setSyncingAll(true);
        try {
            const outOfSyncApps = appDiffs.filter(app => app.syncStatus === 'OutOfSync');
            for (const app of outOfSyncApps) {
                try {
                    await services.applications.sync(app.appName, app.appNamespace, null, false, false, null, null);
                } catch (e) {
                    console.error(`Failed to sync ${app.appName}`, e);
                }
            }
            setRefreshStrategy(undefined);
            setRefreshTrigger(prev => prev + 1);
        } finally {
            setSyncingAll(false);
        }
    };

    const filteredDiffs = appDiffs
        .map(app => {
            const key = `${app.appNamespace}/${app.appName}`;
            const displayApp = lazyData[key] || app;
            const filteredResDiffs = displayApp.diffs.filter(d => {
                const query = searchQuery.toLowerCase().trim();
                if (!query) {
                    return true;
                }
                return d.kind.toLowerCase().includes(query) || (d.namespace || '').toLowerCase().includes(query) || displayApp.appName.toLowerCase().includes(query);
            });

            return {
                ...displayApp,
                diffs: filteredResDiffs
            };
        })
        .filter(app => {
            const query = searchQuery.toLowerCase().trim();
            if (!query) {
                return true;
            }
            return app.appName.toLowerCase().includes(query) || app.diffs.length > 0;
        });

    const totalOutOfSync = targetApps.filter(app => app.status.sync.status === 'OutOfSync').length;
    const totalDriftedResources = appDiffs.reduce((sum, app) => sum + (app.diffs?.length || 0), 0);

    return (
        <SlidingPanel
            isShown={isShown}
            onClose={onClose}
            header={
                <div style={{display: 'flex', alignItems: 'center', gap: '20px'}}>
                    <span style={{fontWeight: 'bold', fontSize: '15px', color: '#364047'}}>
                        Out-of-Sync Apps: <span style={{color: '#e96d76'}}>{totalOutOfSync}</span> | Drifted Resources:{' '}
                        <span style={{color: '#e96d76'}}>{totalDriftedResources}</span>
                    </span>
                    <div>
                        <button className='argo-button argo-button--base' disabled={syncingAll || loading} onClick={handleSyncAllSelected}>
                            {syncingAll ? 'Syncing...' : 'Sync All Selected'}
                        </button>{' '}
                        <button className='argo-button argo-button--base-o' disabled={loading} onClick={handleRefresh}>
                            {loading && refreshStrategy === 'normal' ? 'Refreshing...' : 'Refresh'}
                        </button>
                    </div>
                </div>
            }>
            <div className='global-diff-container'>
                <div style={{display: 'flex', alignItems: 'center', gap: '15px', marginBottom: '15px'}}>
                    <div className='global-diff-search' style={{flex: 1, marginBottom: 0}}>
                        <i className='fa fa-search global-diff-search__icon' />
                        <input
                            type='text'
                            className='argo-field'
                            placeholder='Filter by Kind, Namespace, or Application Name...'
                            value={searchQuery}
                            onChange={e => setSearchQuery(e.target.value)}
                        />
                    </div>
                    <div style={{display: 'flex', gap: '8px'}}>
                        <button className='argo-button argo-button--base-o argo-button--sm' onClick={handleExpandAll}>
                            <i className='fa fa-expand-arrows-alt' style={{marginRight: '5px'}} /> Expand All
                        </button>
                        <button className='argo-button argo-button--base-o argo-button--sm' onClick={handleCollapseAll}>
                            <i className='fa fa-compress-arrows-alt' style={{marginRight: '5px'}} /> Collapse All
                        </button>
                    </div>
                </div>

                {loading ? (
                    <div className='global-diff-loading'>
                        <Spinner show={true} />
                        <span>Loading diff summaries...</span>
                    </div>
                ) : (
                    <div className='global-diff-list'>
                        {filteredDiffs.length > 0 ? (
                            filteredDiffs.map(app => {
                                const key = `${app.appNamespace}/${app.appName}`;
                                const displaySummary = lazyData[key] || app;
                                return (
                                    <ApplicationDiffAccordion
                                        key={key}
                                        appSummary={displaySummary}
                                        isLazy={app.isLazy && !lazyData[key]}
                                        isLoadingLazy={loadingLazy[key]}
                                        isOpen={isAppExpanded(app.appName, app.appNamespace)}
                                        onToggle={() => handleToggleApp(app.appName, app.appNamespace)}
                                        onExpand={() => handleExpandLazy(app.appName, app.appNamespace)}
                                    />
                                );
                            })
                        ) : (
                            <div className='global-diff-empty'>No applications with out-of-sync or drifted resources found matching the criteria.</div>
                        )}
                    </div>
                )}

                {totalPages > 1 && (
                    <div className='global-diff-pagination' style={{display: 'flex', justifyContent: 'center', alignItems: 'center', gap: '10px', marginTop: '1.5rem'}}>
                        <button
                            className='argo-button argo-button--base-o argo-button--sm'
                            disabled={currentPage === 1}
                            onClick={() => setCurrentPage(prev => Math.max(1, prev - 1))}>
                            Previous
                        </button>
                        <span>
                            Page {currentPage} of {totalPages} (showing {pageApps.length} of {targetApps.length} apps)
                        </span>
                        <button
                            className='argo-button argo-button--base-o argo-button--sm'
                            disabled={currentPage === totalPages}
                            onClick={() => setCurrentPage(prev => Math.min(totalPages, prev + 1))}>
                            Next
                        </button>
                    </div>
                )}
            </div>
        </SlidingPanel>
    );
};
