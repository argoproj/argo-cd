import * as React from 'react';
import * as models from '../../../shared/models';
import { services } from '../../../shared/services';
import { Page, Spinner } from '../../../shared/components';
import { ApplicationResourcesDiff } from '../application-resources-diff/application-resources-diff';
import { ComparisonStatusIcon, HealthStatusIcon } from '../utils';
import './global-diffs-view.scss';

// Helper to create a namespace-qualified identifier for an Application
const getAppInstanceId = (app: models.Application): string => {
    const ns = app.metadata.namespace || 'default';
    return `${ns}/${app.metadata.name}`;
};

// Helper to extract application name from qualified ID
const parseAppId = (appId: string): string => {
    return appId.split('/').slice(-1)[0];
};

interface GlobalDiffsViewState {
    loading: boolean;
    apps: models.Application[];
    checkedApps: Set<string>;          // now stores qualified IDs (namespace/name)
    diffs: { [appId: string]: models.ResourceDiff[] }; // keyed by qualified ID
    error: Error | null;
    selectedKinds: Set<string>;
    expandedApps: Set<string>;         // now stores qualified IDs
}

export const GlobalDiffsView = (props: any) => {
    const [state, setState] = React.useState<GlobalDiffsViewState>({
        loading: true,
        apps: [],
        checkedApps: new Set(),
        diffs: {},
        error: null,
        selectedKinds: new Set(),
        expandedApps: new Set()
    });

    const query = new URLSearchParams(props.location?.search || '');
    const projects = query.get('proj')?.split(',').filter(Boolean) || [];
    const selector = query.get('labels') || '';
    const destNamespaceFilter = query.get('namespace') || '';
    const nameFilter = query.get('apps')?.split(',').filter(Boolean) || [];

    const appFields = ['items.metadata.name', 'items.metadata.namespace', 'items.spec.project', 'items.spec.destination', 'items.status.sync.status', 'items.status.health'];

    const applicationMatchesNamespaceFilter = (app: models.Application): boolean => {
        if (!destNamespaceFilter) return true;
        return app.spec.destination.namespace === destNamespaceFilter;
    };

    // Step 2: Fetch batch diffs for checked applications
    const fetchDiffsForApps = (checked: Set<string>) => {
        if (checked.size === 0) {
            setState(prev => ({ ...prev, diffs: {}, selectedKinds: new Set(), loading: false }));
            return;
        }

        setState(prev => ({ ...prev, loading: true }));

        // When sending to backend, we need to send either names or qualified IDs
        // For backward compatibility, we'll send both formats if needed
        const backendNames = new Set<string>();
        checked.forEach(id => {
            // For qualified IDs, extract just the name part for backward compatibility
            const name = parseAppId(id);
            backendNames.add(name);
        });

        services.applications
            .batchManagedResources({ applicationNames: Array.from(backendNames) })
            .then(batchItems => {
                const diffsMap: { [appId: string]: models.ResourceDiff[] } = {};
                const kinds = new Set<string>;

                batchItems.forEach(item => {
                    // The applicationName field now contains the qualified ID (namespace/name)
                    // since we updated the server to return it in that format
                    const qualifiedId = item.applicationName;
                    diffsMap[qualifiedId] = item.items || [];
                    (item.items || []).forEach(diffItem => {
                        if (diffItem.kind) {
                            kinds.add(diffItem.kind);
                        }
                    });
                });

                setState(prev => ({
                    ...prev,
                    diffs: diffsMap,
                    selectedKinds: kinds, // select all kinds by default
                    loading: false
                }));
            })
            .catch(err => {
                setState(prev => ({ ...prev, loading: false, error: err }));
            });
    };

    // Step 1: Fetch matching applications on mount
    React.useEffect(() => {
        // If test-provided apps are passed in props, skip fetching and use them directly
        if (props.apps) {
            setState(prev => ({ ...prev, apps: props.apps as models.Application[], loading: false }));
            return;
        }
        let isMounted = true;

        services.applications
            .list(projects, 'application', { selector, fields: appFields })
            .then(appList => {
                if (!isMounted) return;

                // Filter to OutOfSync applications
                let filteredApps = (appList.items || []) as models.Application[];
                filteredApps = filteredApps.filter(app =>
                    app.status.sync.status === 'OutOfSync' &&
                    applicationMatchesNamespaceFilter(app)
                );

                // Filter by app names if explicitly specified in query parameters
                if (nameFilter.length > 0) {
                    const nameSet = new Set(nameFilter);
                    filteredApps = filteredApps.filter(app =>
                        nameSet.has(app.metadata.name)
                    );
                }

                // Initial selection: check the first 10 apps to avoid performance overload
                const initialChecked = new Set<string>();
                filteredApps.slice(0, 10).forEach(app => {
                    initialChecked.add(getAppInstanceId(app));
                });

                // Initially expand all checked apps
                const initialExpanded = new Set<string>(initialChecked);

                setState(prev => ({
                    ...prev,
                    apps: filteredApps,
                    checkedApps: initialChecked,
                    expandedApps: initialExpanded,
                    loading: filteredApps.length === 0 ? false : prev.loading
                }));

                if (filteredApps.length > 0) {
                    fetchDiffsForApps(initialChecked);
                }
            })
            .catch(err => {
                if (!isMounted) return;
                setState(prev => ({ ...prev, loading: false, error: err }));
            });

        return () => {
            isMounted = false;
        };
    }, [props.location.search, destNamespaceFilter, nameFilter, props.apps]); // Added dependencies

    const handleCheckboxChange = (appId: string) => {
        const newChecked = new Set(state.checkedApps);
        if (newChecked.has(appId)) {
            newChecked.delete(appId);
        } else {
            newChecked.add(appId);
        }

        // Keep expanded state in sync with checking/unchecking
        const newExpanded = new Set(state.expandedApps);
        if (newChecked.has(appId)) {
            newExpanded.add(appId);
        } else {
            newExpanded.delete(appId);
        }

        setState(prev => ({ ...prev, checkedApps: newChecked, expandedApps: newExpanded }));
        fetchDiffsForApps(newChecked);
    };

    const handleSelectAllApps = (select: boolean) => {
        const newChecked = new Set<string>();
        const newExpanded = new Set<string>();
        if (select) {
            // Use the current apps in state to select up to 15
            const appsToSelect = [...state.apps].slice(0, 15);
            appsToSelect.forEach(app => {
                const appId = getAppInstanceId(app);
                newChecked.add(appId);
                newExpanded.add(appId);
            });
        }
        setState(prev => ({ ...prev, checkedApps: newChecked, expandedApps: newExpanded }));
        fetchDiffsForApps(newChecked);
    };

    const handleToggleKind = (kind: string) => {
        const newKinds = new Set(state.selectedKinds);
        if (newKinds.has(kind)) {
            newKinds.delete(kind);
        } else {
            newKinds.add(kind);
        }
        setState(prev => ({ ...prev, selectedKinds: newKinds }));
    };

    const handleSelectAllKinds = (select: boolean) => {
        const allKinds = new Set<string>();
        if (select) {
            Object.values(state.diffs).forEach(items => {
                items.forEach(item => {
                    if (item.kind) allKinds.add(item.kind);
                });
            });
        }
        setState(prev => ({ ...prev, selectedKinds: allKinds }));
    };

    const handleToggleExpandApp = (appId: string) => {
        const newExpanded = new Set(state.expandedApps);
        if (newExpanded.has(appId)) {
            newExpanded.delete(appId);
        } else {
            newExpanded.add(appId);
        }
        setState(prev => ({ ...prev, expandedApps: newExpanded }));
    };

    const handleExpandAllApps = (expand: boolean) => {
        const newExpanded = new Set<string>();
        if (expand) {
            state.apps.forEach(app => {
                const appId = getAppInstanceId(app);
                if (state.checkedApps.has(appId)) {
                    newExpanded.add(appId);
                }
            });
        }
        setState(prev => ({ ...prev, expandedApps: newExpanded }));
    };

    // Calculate unique kinds currently available in loaded diffs
    const availableKinds = React.useMemo(() => {
        const kindsMap: { [kind: string]: number } = {};
        Object.entries(state.diffs).forEach(([appId, items]) => {
            if (!state.checkedApps.has(appId)) return;
            items.forEach(item => {
                if (item.kind) {
                    kindsMap[item.kind] = (kindsMap[item.kind] || 0) + 1;
                }
            });
        });
        return kindsMap;
    }, [state.diffs, state.checkedApps]);

    return (
        <Page
            title='Global Diffs'
            toolbar={{
                breadcrumbs: [{ title: 'Applications', path: '/applications' }, { title: 'Global Diffs' }]
            }}>
            <div className='global-diffs-container'>
                {/* Active Filters Summary */}
                <div className='global-diffs-card filter-summary-card'>
                    <div className='filter-summary-title'>
                        <i className='fa fa-filter' /> Active Filters
                    </div>
                    <div className='filter-pills-list'>
                        {projects.length > 0 && (
                            <span className='filter-pill'>
                                <strong>Projects:</strong> {projects.join(', ')}
                            </span>
                        )}
                        {selector && (
                            <span className='filter-pill'>
                                <strong>Labels:</strong> {selector}
                            </span>
                        )}
                        {destNamespaceFilter && (
                            <span className='filter-pill'>
                                <strong>Namespace:</strong> {destNamespaceFilter}
                            </span>
                        )}
                        {nameFilter.length > 0 && (
                            <span className='filter-pill'>
                                <strong>Selected Apps:</strong> {nameFilter.join(', ')}
                            </span>
                        )}
                        {projects.length === 0 && !selector && !destNamespaceFilter && nameFilter.length === 0 && (
                            <span className='filter-pill filter-pill--all'>Showing all applications in cluster</span>
                        )}
                    </div>
                </div>

                {/* Main Controls Grid */}
                <div className='global-diffs-grid'>
                    {/* Left: Applications Selector Panel */}
                    <div className='global-diffs-card apps-selector-card'>
                        <div className='card-header-bar'>
                            <h3>Applications ({state.apps.length})</h3>
                            <div className='quick-actions'>
                                <button id='btn-select-all' className='action-link' onClick={() => handleSelectAllApps(true)}>
                                    Select First 15
                                </button>
                                <span>/</span>
                                <button id='btn-select-none' className='action-link' onClick={() => handleSelectAllApps(false)}>
                                    Clear
                                </button>
                            </div>
                        </div>

                        {state.apps.length === 0 ? (
                            <div className='no-apps-message'>No OutOfSync applications match the current filter.</div>
                        ) : (
                            <div className='apps-list-checkboxes'>
                                {state.apps.length > 10 && (
                                    <div className='warning-banner'>
                                        <i className='fa fa-exclamation-triangle' /> Only showing first 10 apps by default to protect browser performance.
                                    </div>
                                )}
                                {state.apps.map(app => {
                                    const appId = getAppInstanceId(app);
                                    const isChecked = state.checkedApps.has(appId);
                                    return (
                                        <label key={appId} className={`app-checkbox-label ${isChecked ? 'checked' : ''}`}>
                                            <input type='checkbox' id={`chk-${appId}`} checked={isChecked} onChange={() => handleCheckboxChange(appId)} />
                                            <span className='app-status-icons'>
                                                <ComparisonStatusIcon status={app.status.sync.status} />
                                                <HealthStatusIcon state={app.status.health} />
                                            </span>
                                            <span className='app-checkbox-text'>{app.metadata.name}</span>
                                        </label>
                                    );
                                })}
                            </div>
                        )}
                    </div>

                    {/* Right: Diffs Viewer Panel */}
                    <div className='global-diffs-main-panel'>
                        {/* Resource Kinds Filter & Global Collapse/Expand */}
                        {state.checkedApps.size > 0 && Object.keys(availableKinds).length > 0 && (
                            <div className='global-diffs-card kinds-filter-card'>
                                <div className='kinds-filter-title'>
                                    <span>Filter by Resource Kind</span>
                                    <div className='quick-actions'>
                                        <button id='btn-kinds-all' className='action-link' onClick={() => handleSelectAllKinds(true)}>
                                            All
                                        </button>
                                        <span>/</span>
                                        <button id='btn-kinds-none' className='action-link' onClick={() => handleSelectAllKinds(false)}>
                                            None
                                        </button>
                                    </div>
                                </div>
                                <div className='kinds-pills-list'>
                                    {Object.entries(availableKinds).map(([kind, count]) => {
                                        const isSelected = state.selectedKinds.has(kind);
                                        return (
                                            <button
                                                key={kind}
                                                id={`btn-kind-${kind}`}
                                                className={`kind-pill ${isSelected ? 'selected' : ''}`}
                                                onClick={() => handleToggleKind(kind)}>
                                                {kind} <span className='kind-count'>{count}</span>
                                            </button>
                                        );
                                    })}
                                </div>

                                <div className='diff-layout-actions'>
                                    <button id='btn-expand-all' className='argo-button argo-button--base-o' onClick={() => handleExpandAllApps(true)}>
                                        <i className='fa fa-chevron-down' /> Expand Checked
                                    </button>
                                    <button id='btn-collapse-all' className='argo-button argo-button--base-o' onClick={() => handleExpandAllApps(false)}>
                                        <i className='fa fa-chevron-right' /> Collapse All
                                    </button>
                                </div>
                            </div>
                        )}

                        {/* Spinner for Loading */}
                        {state.loading && (
                            <div className='loading-overlay'>
                                <Spinner show={true} />
                                <span className='loading-text'>Loading diff state from API server...</span>
                            </div>
                        )}

                        {/* Diffs List */}
                        {!state.loading && !state.error && state.checkedApps.size === 0 && (
                            <div className='empty-diffs-state'>
                                <i className='fa fa-check-circle' />
                                <h3>No applications selected</h3>
                                <p>Check one or more applications from the left panel to load and view their drift manifests.</p>
                            </div>
                        )}

                        {state.error && (
                            <div className='error-banner-global'>
                                <i className='fa fa-exclamation-circle' />
                                <h3>Failed to load managed resource diffs</h3>
                                <p>{state.error.message || 'An unexpected error occurred.'}</p>
                            </div>
                        )}

                        {!state.loading && !state.error && state.checkedApps.size > 0 && (
                            <div className='apps-diffs-list'>
                                {Array.from(state.checkedApps).map(appId => {
                                    const app = state.apps.find(a => getAppInstanceId(a) === appId);
                                    const rawDiffs = state.diffs[appId] || [];
                                    const filteredDiffs = rawDiffs.filter(d => state.selectedKinds.has(d.kind));
                                    const isExpanded = state.expandedApps.has(appId);

                                    if (!app) return null;

                                    return (
                                        <div key={appId} className={`global-diffs-card app-diff-section-card ${isExpanded ? 'expanded' : ''}`}>
                                            <div className='app-diff-header' onClick={() => handleToggleExpandApp(appId)}>
                                                <div className='app-diff-header-left'>
                                                    <i className={`fa fa-chevron-${isExpanded ? 'down' : 'right'} toggle-arrow`} />
                                                    <span className='app-name-title'>{app.metadata.name}</span>
                                                    <span className='app-meta-badge proj-badge'>{app.spec.project}</span>
                                                    <span className='app-meta-badge ns-badge'>{app.spec.destination.namespace || 'default'}</span>
                                                </div>
                                                <div className='app-diff-header-right'>
                                                    <span className='diff-count-badge'>
                                                        {filteredDiffs.length} / {rawDiffs.length} diffs
                                                    </span>
                                                    <ComparisonStatusIcon status={app.status.sync.status} />
                                                    <HealthStatusIcon state={app.status.health} />
                                                </div>
                                            </div>

                                            {isExpanded && (
                                                <div className='app-diff-body'>
                                                    {filteredDiffs.length === 0 ? (
                                                        <div className='no-diffs-detail'>
                                                            {rawDiffs.length === 0
                                                                ? 'No drift detected in the managed resources.'
                                                                : 'All diffs are filtered out by current resource kind filters.'}
                                                        </div>
                                                    ) : (
                                                        <ApplicationResourcesDiff states={filteredDiffs} />
                                                    )}
                                                </div>
                                            )}
                                        </div>
                                    );
                                })}
                            </div>
                        )}
                    </div>
                </div>
            </div>
        </Page>
    );
};