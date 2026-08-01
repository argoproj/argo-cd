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
}

export const GlobalDiffModal = (props: GlobalDiffModalProps) => {
    const {isShown, onClose, appNames, projects, selector} = props;
    const [loading, setLoading] = React.useState(false);
    const [appDiffs, setAppDiffs] = React.useState<models.ApplicationDiffSummary[]>([]);
    const [searchQuery, setSearchQuery] = React.useState('');
    const [syncingAll, setSyncingAll] = React.useState(false);
    const [refreshTrigger, setRefreshTrigger] = React.useState(0);
    const [refreshStrategy, setRefreshStrategy] = React.useState<string | undefined>(undefined);

    React.useEffect(() => {
        if (isShown) {
            let active = true;
            Promise.resolve().then(() => {
                if (active) {
                    setLoading(true);
                }
            });
            services.applications
                .getBatchApplicationDiff({
                    appNames,
                    projects,
                    selector,
                    refresh: refreshStrategy
                })
                .then(data => {
                    if (active) {
                        setAppDiffs(data);
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
        }
    }, [isShown, appNames, projects, selector, refreshTrigger, refreshStrategy]);

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

    // Filter logic by Kind, Namespace, or Application Name
    const filteredDiffs = appDiffs
        .map(app => {
            const filteredResDiffs = app.diffs.filter(d => {
                const query = searchQuery.toLowerCase().trim();
                if (!query) {
                    return true;
                }
                return d.kind.toLowerCase().includes(query) || (d.namespace || '').toLowerCase().includes(query) || app.appName.toLowerCase().includes(query);
            });

            return {
                ...app,
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

    const totalOutOfSyncApps = appDiffs.filter(app => app.syncStatus === 'OutOfSync').length;
    const totalDriftedResources = appDiffs.reduce((acc, app) => acc + app.diffs.length, 0);

    return (
        <SlidingPanel
            isShown={isShown}
            onClose={onClose}
            header={
                <div className='global-diff-header'>
                    <div className='global-diff-header__stats'>
                        <span className='global-diff-header__stat-item'>
                            <strong>{totalOutOfSyncApps}</strong> Out-of-sync App{totalOutOfSyncApps !== 1 ? 's' : ''}
                        </span>
                        <span className='global-diff-header__stat-item'>
                            <strong>{totalDriftedResources}</strong> Drifted Resource{totalDriftedResources !== 1 ? 's' : ''}
                        </span>
                    </div>
                    <div className='global-diff-header__actions'>
                        <button className='argo-button argo-button--base' disabled={syncingAll || loading} onClick={handleSyncAllSelected}>
                            {syncingAll ? 'Syncing...' : 'Sync All Selected'}
                        </button>
                        <button className='argo-button argo-button--base-o' disabled={loading} onClick={handleRefresh}>
                            {loading && refreshStrategy === 'normal' ? 'Refreshing...' : 'Refresh'}
                        </button>
                        <button className='argo-button argo-button--base-o' onClick={onClose}>
                            Close
                        </button>
                    </div>
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
                            filteredDiffs.map(app => <ApplicationDiffAccordion key={app.appName} appSummary={app} />)
                        ) : (
                            <div className='global-diff-empty'>No applications with out-of-sync or drifted resources found matching the criteria.</div>
                        )}
                    </div>
                )}
            </div>
        </SlidingPanel>
    );
};
