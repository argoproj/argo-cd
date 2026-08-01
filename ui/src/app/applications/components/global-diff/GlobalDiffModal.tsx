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
    appNames?: string[];
    projects?: string[];
    selector?: string;
    allApps?: models.Application[];
}

const PAGE_SIZE = 50;

export const GlobalDiffModal = (props: GlobalDiffModalProps) => {
    const {isShown, onClose, appNames, projects, selector, allApps} = props;
    const [loading, setLoading] = React.useState(false);
    const [appDiffs, setAppDiffs] = React.useState<models.ApplicationDiffSummary[]>([]);
    const [searchQuery, setSearchQuery] = React.useState('');
    const [syncingAll, setSyncingAll] = React.useState(false);
    const [refreshTrigger, setRefreshTrigger] = React.useState(0);
    const [refreshStrategy, setRefreshStrategy] = React.useState<string | undefined>(undefined);
    const [currentPage, setCurrentPage] = React.useState(1);

    const [lazyData, setLazyData] = React.useState<Record<string, models.ApplicationDiffSummary>>({});
    const [loadingLazy, setLoadingLazy] = React.useState<Record<string, boolean>>({});

    const targetApps = React.useMemo(() => {
        let list = allApps || [];
        if (appNames && appNames.length > 0) {
            list = list.filter(app => appNames.includes(app.metadata.name));
        } else {
            list = list.filter(app => {
                if (projects && projects.length > 0 && !projects.includes(app.spec.project)) {
                    return false;
                }
                return app.status.sync.status === 'OutOfSync';
            });
        }
        return list;
    }, [allApps, appNames, projects]);

    const totalPages = Math.ceil(targetApps.length / PAGE_SIZE);
    const pageApps = React.useMemo(() => {
        const start = (currentPage - 1) * PAGE_SIZE;
        return targetApps.slice(start, start + PAGE_SIZE);
    }, [targetApps, currentPage]);

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
    }, [isShown, appNames, projects, selector]);

    React.useEffect(() => {
        if (isShown && pageApps.length > 0) {
            let active = true;
            Promise.resolve().then(() => {
                if (active) {
                    setLoading(true);
                }
            });

            const pageAppNames = pageApps.map(app => app.metadata.name);
            services.applications
                .getBatchApplicationDiff({
                    appNames: pageAppNames,
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
    }, [isShown, pageApps, refreshTrigger, refreshStrategy, projects, selector]);

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

    const handleExpandLazy = async (appName: string) => {
        if (lazyData[appName]) {
            return;
        }
        setLoadingLazy(prev => ({...prev, [appName]: true}));
        try {
            const result = await services.applications.getBatchApplicationDiff({
                appNames: [appName]
            });
            if (result && result.length > 0) {
                setLazyData(prev => ({...prev, [appName]: result[0]}));
            }
        } catch (err) {
            console.error(`Failed to load lazy diff for ${appName}`, err);
        } finally {
            setLoadingLazy(prev => ({...prev, [appName]: false}));
        }
    };

    const filteredDiffs = appDiffs
        .map(app => {
            const displayApp = lazyData[app.appName] || app;
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

    return (
        <SlidingPanel
            isShown={isShown}
            onClose={onClose}
            header={
                <div>
                    <button className='argo-button argo-button--base' disabled={syncingAll || loading} onClick={handleSyncAllSelected}>
                        {syncingAll ? 'Syncing...' : 'Sync All Selected'}
                    </button>{' '}
                    <button className='argo-button argo-button--base-o' disabled={loading} onClick={handleRefresh}>
                        {loading && refreshStrategy === 'normal' ? 'Refreshing...' : 'Refresh'}
                    </button>
                </div>
            }>
            <div className='global-diff-container'>
                <div className='global-diff-search'>
                    <i className='fa fa-search global-diff-search__icon' />
                    <input
                        type='text'
                        className='argo-field'
                        placeholder='Filter by Kind, Namespace, or Application Name...'
                        value={searchQuery}
                        onChange={e => setSearchQuery(e.target.value)}
                    />
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
                                const displaySummary = lazyData[app.appName] || app;
                                return (
                                    <ApplicationDiffAccordion
                                        key={app.appName}
                                        appSummary={displaySummary}
                                        isLazy={app.isLazy && !lazyData[app.appName]}
                                        isLoadingLazy={loadingLazy[app.appName]}
                                        onExpand={() => handleExpandLazy(app.appName)}
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
