import {HelpIcon} from 'argo-ui';
import * as React from 'react';
import {ARGO_GRAY6_COLOR, DataLoader} from '../../../shared/components';
import {Revision} from '../../../shared/components/revision';
import {revisionUrl} from '../../../shared/components/urls';
import {Timestamp} from '../../../shared/components/timestamp';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {
    ApplicationSyncWindowStatusIcon,
    ComparisonStatusIcon,
    formatApplicationSetProgressiveSyncStep,
    getAppDefaultSource,
    getAppDefaultSyncRevisionExtra,
    getSyncRevisionLabelSuffix,
    getAppOperationState,
    getHealthStatusColor,
    HydrateOperationPhaseIcon,
    hydrationStatusMessage,
    getProgressiveSyncStatusColor,
    getProgressiveSyncStatusIcon
} from '../utils';
import {getConditionCategory, HealthStatusIcon, OperationState, syncStatusMessage, getAppDefaultSyncRevision, getAppDefaultOperationSyncRevision} from '../utils';
import {ConditionCounters} from './condition-counters';
import {RevisionMetadataPanel} from './revision-metadata-panel';
import * as utils from '../utils';
import {COLORS} from '../../../shared/components/colors';

import './application-status-panel.scss';

interface Props {
    application: models.Application;
    collapsed?: boolean;
    showDiff?: () => any;
    showOperation?: () => any;
    showHydrateOperation?: () => any;
    showConditions?: () => any;
    showExtension?: (id: string) => any;
    showMetadataInfo?: (revision: string) => any;
}

interface SectionInfo {
    title: string;
    helpContent?: string;
}

const sectionLabel = (info: SectionInfo) => (
    <label style={{display: 'flex', alignItems: 'flex-start', fontSize: '12px', fontWeight: 600, color: ARGO_GRAY6_COLOR, minHeight: '18px'}}>
        {info.title}
        {info.helpContent && (
            <span style={{marginLeft: '5px'}}>
                <HelpIcon title={info.helpContent} />
            </span>
        )}
    </label>
);

const sectionHeader = (info: SectionInfo, onClick?: () => any) => {
    return (
        <div style={{display: 'flex', alignItems: 'center'}}>
            {sectionLabel(info)}
            {onClick && (
                <button className='argo-button application-status-panel__more-button' onClick={onClick}>
                    <i className='fa fa-ellipsis-h' />
                </button>
            )}
        </div>
    );
};

const getApplicationSetOwnerRef = (application: models.Application) => {
    return application.metadata.ownerReferences?.find(ref => ref.kind === 'ApplicationSet');
};

const renderSyncStatusRevision = (application: models.Application) => {
    const source = getAppDefaultSource(application);
    if (!source) {
        return syncStatusMessage(application);
    }

    const status = application.status.sync.status;
    if (status !== models.SyncStatuses.Synced && status !== models.SyncStatuses.OutOfSync) {
        return syncStatusMessage(application);
    }

    const prefix = status === models.SyncStatuses.Synced ? 'to' : 'from';
    const branchName = source.targetRevision ? (source.tagPrefix || '') + source.targetRevision : 'HEAD';
    const resolvedRevision = application.status.sync.revision || branchName;
    const extra = getAppDefaultSyncRevisionExtra(application);
    const revisionLink = revisionUrl(source.repoURL, resolvedRevision, false);

    const shortRevision = getSyncRevisionLabelSuffix(source.repoURL, branchName, resolvedRevision, source.chart);

    const revisionContent = (
        <>
            <span className='application-status-panel__item-value__revision-branch'>{branchName}</span>
            {shortRevision && <span className='application-status-panel__item-value__revision-sha'> ({shortRevision})</span>}
        </>
    );

    return (
        <>
            <span className='application-status-panel__item-value__revision-prefix'>{prefix}</span>
            <span className='application-status-panel__item-value__revision-main'>
                {revisionLink ? (
                    <a href={revisionLink} target='_blank' rel='noopener noreferrer' title={branchName}>
                        {revisionContent}
                        <i className='fa fa-external-link-alt' />
                    </a>
                ) : (
                    <span title={branchName}>{revisionContent}</span>
                )}
            </span>
            {extra && <span className='application-status-panel__item-value__revision-extra'>{extra.trimStart()}</span>}
        </>
    );
};

const ProgressiveSyncStatus = ({application}: {application: models.Application}) => {
    const appSetRef = getApplicationSetOwnerRef(application);
    if (!appSetRef) {
        return null;
    }

    return (
        <DataLoader
            input={application}
            errorRenderer={() => {
                // For any errors, show a minimal error state
                return (
                    <div className='application-status-panel__item'>
                        {sectionHeader({
                            title: 'PROGRESSIVE SYNC',
                            helpContent: 'Shows the current status of progressive sync for applications managed by an ApplicationSet.'
                        })}
                        <div className='application-status-panel__item-value'>
                            <i className='fa fa-exclamation-triangle' style={{color: COLORS.sync.unknown}} /> Error
                        </div>
                        <div className='application-status-panel__item-name'>Unable to load Progressive Sync status</div>
                    </div>
                );
            }}
            load={async () => {
                // Find ApplicationSet by searching all namespaces dynamically
                const appSetList = await services.applications.listApplicationSets();
                const appSet = appSetList.items?.find(item => item.metadata.name === appSetRef.name);

                return {appSet};
            }}>
            {({appSet}: {appSet: models.ApplicationSet}) => {
                // Hide panel if: Progressive Sync disabled, no permission, or not RollingSync strategy
                if (!appSet || !appSet.status?.applicationStatus || appSet?.spec?.strategy?.type !== 'RollingSync') {
                    return null;
                }

                // Get the current application's status from the ApplicationSet applicationStatus
                const appResource = appSet.status?.applicationStatus?.find(status => status.application === application.metadata.name);

                // If no application status is found, show a default status
                if (!appResource) {
                    return (
                        <div className='application-status-panel__item'>
                            {sectionHeader({
                                title: 'PROGRESSIVE SYNC',
                                helpContent: 'Shows the current status of progressive sync for applications managed by an ApplicationSet with RollingSync strategy.'
                            })}
                            <div className='application-status-panel__item-value'>
                                <i className='fa fa-clock' style={{color: COLORS.sync.out_of_sync}} /> Waiting
                            </div>
                            <div className='application-status-panel__item-name'>Application status not yet available from ApplicationSet</div>
                        </div>
                    );
                }

                // Get last transition time from application status
                const lastTransitionTime = appResource?.lastTransitionTime;

                return (
                    <div className='application-status-panel__item'>
                        {sectionHeader({
                            title: 'PROGRESSIVE SYNC',
                            helpContent: 'Shows the current status of progressive sync for applications managed by an ApplicationSet with RollingSync strategy.'
                        })}
                        <div className='application-status-panel__item-value' style={{color: getProgressiveSyncStatusColor(appResource.status)}}>
                            {getProgressiveSyncStatusIcon({status: appResource.status})}&nbsp;{appResource.status}
                        </div>
                        {appResource?.step !== undefined && <div className='application-status-panel__item-value'>{formatApplicationSetProgressiveSyncStep(appResource.step)}</div>}
                        {lastTransitionTime && (
                            <div className='application-status-panel__item-name' style={{marginBottom: '0.5em'}}>
                                Last Transition: <br />
                                <Timestamp date={lastTransitionTime} />
                            </div>
                        )}
                        {appResource?.message && <div className='application-status-panel__item-name'>{appResource.message}</div>}
                    </div>
                );
            }}
        </DataLoader>
    );
};

export const ApplicationStatusPanel = ({application, collapsed, showDiff, showOperation, showHydrateOperation, showConditions, showExtension, showMetadataInfo}: Props) => {
    // Keep the full panel mounted once shown: unmounting it on collapse would re-run its
    // data loaders (and re-fire their requests) on every expand.
    const [everExpanded, setEverExpanded] = React.useState(!collapsed);
    if (!collapsed && !everExpanded) {
        setEverExpanded(true);
    }

    // While collapsed, keep feeding the hidden input-driven loaders (progressive sync,
    // sync windows) the last visible application so app updates do not trigger their
    // requests; they refresh once on expand.
    const [visibleApplication, setVisibleApplication] = React.useState(application);
    if (!collapsed && visibleApplication !== application) {
        setVisibleApplication(application);
    }

    // Sync windows are time-based, so re-expanding must refetch them even when the
    // application itself is unchanged.
    const [prevCollapsed, setPrevCollapsed] = React.useState(collapsed);
    const [expandCount, setExpandCount] = React.useState(0);
    if (prevCollapsed !== collapsed) {
        setPrevCollapsed(collapsed);
        if (!collapsed) {
            setExpandCount(expandCount + 1);
        }
    }

    // Only show Progressive Sync if the application has an ApplicationSet parent
    // The actual strategy validation will be done inside ProgressiveSyncStatus component
    const showProgressiveSync = !!getApplicationSetOwnerRef(visibleApplication);

    const today = new Date();

    let daysSinceLastSynchronized = 0;
    const history = application.status.history || [];
    if (history.length > 0) {
        const deployDate = new Date(history[history.length - 1].deployedAt);
        daysSinceLastSynchronized = Math.round(Math.abs((today.getTime() - deployDate.getTime()) / (24 * 60 * 60 * 1000)));
    }
    const cntByCategory = (application.status.conditions || []).reduce(
        (map, next) => map.set(getConditionCategory(next), (map.get(getConditionCategory(next)) || 0) + 1),
        new Map<string, number>()
    );
    const appOperationState = getAppOperationState(application);

    // Unknown's palette color is too low-contrast for text; inherit the themed color there
    const healthTextColor = application.status.health.status === models.HealthStatuses.Unknown ? undefined : getHealthStatusColor(application.status.health.status);

    const statusExtensions = services.extensions.getStatusPanelExtensions();

    const operationStateRevision = getAppDefaultOperationSyncRevision(application);
    const infos = cntByCategory.get('info');
    const warnings = cntByCategory.get('warning');
    const errors = cntByCategory.get('error');
    const source = getAppDefaultSource(application);
    // Metadata loader inputs come from the frozen snapshot so hidden loaders do not
    // remount while collapsed; visible revision text stays live.
    const visibleSource = getAppDefaultSource(visibleApplication);
    const visibleRevision = getAppDefaultSyncRevision(visibleApplication);
    const visibleOperationStateRevision = getAppDefaultOperationSyncRevision(visibleApplication);
    const visibleVersionId = utils.getAppCurrentVersion(visibleApplication);
    const visibleAppOperationState = getAppOperationState(visibleApplication);
    const visibleHasMultipleSources = visibleApplication.spec.sources?.length > 0;
    const revisionType = visibleSource?.repoURL?.startsWith('oci://') ? 'oci' : visibleSource?.chart ? 'helm' : 'git';

    const conditionSummary = (infos || warnings || errors) && (
        <div className='application-status-panel__collapsed-item application-status-panel__conditions' onClick={() => showConditions && showConditions()}>
            <ConditionCounters infos={infos} warnings={warnings} errors={errors} />
        </div>
    );

    const collapsedSummary = collapsed && (
        <div className='application-status-panel application-status-panel--collapsed row'>
            <div className='application-status-panel__collapsed-item' title='App Health' style={{color: healthTextColor}}>
                <HealthStatusIcon state={application.status.health} />
                &nbsp;
                {application.status.health.status}
            </div>
            {application.spec.sourceHydrator && application.status?.sourceHydrator?.currentOperation && (
                <div className='application-status-panel__collapsed-item' title='Source Hydrator'>
                    <a onClick={() => showHydrateOperation && showHydrateOperation()}>
                        <HydrateOperationPhaseIcon operationState={application.status.sourceHydrator.currentOperation} isButton={true} />
                        &nbsp;
                        {application.status.sourceHydrator.currentOperation.phase}
                    </a>
                </div>
            )}
            <div className='application-status-panel__collapsed-item' title='Sync Status'>
                {application.status.sync.status === models.SyncStatuses.OutOfSync ? (
                    <a onClick={() => showDiff && showDiff()}>
                        <ComparisonStatusIcon status={application.status.sync.status} label={true} isButton={true} />
                    </a>
                ) : (
                    // <span> keeps the icon/label spacing: a bare space between flex children is dropped
                    <span>
                        <ComparisonStatusIcon status={application.status.sync.status} label={true} />
                    </span>
                )}
            </div>
            {appOperationState && (
                <div
                    className={`application-status-panel__collapsed-item application-status-panel__item-value--${appOperationState.phase}`}
                    title={'Last Sync Result: ' + appOperationState.phase}>
                    {application.status.operationState ? (
                        <a onClick={() => showOperation && showOperation()}>
                            <OperationState app={application} isButton={true} />
                        </a>
                    ) : (
                        <span>
                            <OperationState app={application} />
                        </span>
                    )}
                </div>
            )}
            {conditionSummary}
        </div>
    );

    return (
        <React.Fragment>
            {collapsedSummary}
            {(!collapsed || everExpanded) && (
                <div className='application-status-panel row' style={collapsed ? {display: 'none'} : undefined}>
                    <div className='application-status-panel__item'>
                        {sectionHeader({title: 'APP HEALTH', helpContent: 'The health status of your app'})}
                        <div className='application-status-panel__item-value' style={{color: healthTextColor}}>
                            <HealthStatusIcon state={application.status.health} />
                            &nbsp;
                            {application.status.health.status}
                        </div>
                        {application.status.health.message && <div className='application-status-panel__item-name'>{application.status.health.message}</div>}
                    </div>
                    {visibleApplication.spec.sourceHydrator && visibleApplication.status?.sourceHydrator?.currentOperation && (
                        <div className='application-status-panel__item'>
                            <div style={{lineHeight: '19.5px', marginBottom: '0.3em'}}>
                                {sectionLabel({
                                    title: 'SOURCE HYDRATOR',
                                    helpContent: 'The source hydrator reads manifests from git, hydrates (renders) them, and pushes them to a different location in git.'
                                })}
                            </div>
                            {application.spec.sourceHydrator && application.status?.sourceHydrator?.currentOperation && (
                                <React.Fragment>
                                    <div className='application-status-panel__item-value'>
                                        <a className='application-status-panel__item-value__hydrator-link' onClick={() => showHydrateOperation && showHydrateOperation()}>
                                            <HydrateOperationPhaseIcon operationState={application.status.sourceHydrator.currentOperation} isButton={true} />
                                            &nbsp;
                                            {application.status.sourceHydrator.currentOperation.phase}
                                        </a>
                                        <div className='application-status-panel__item-value__revision show-for-large'>{hydrationStatusMessage(application)}</div>
                                    </div>
                                    <div className='application-status-panel__item-name' style={{marginBottom: '0.5em'}}>
                                        {application.status.sourceHydrator.currentOperation.phase}{' '}
                                        <Timestamp
                                            date={application.status.sourceHydrator.currentOperation.finishedAt || application.status.sourceHydrator.currentOperation.startedAt}
                                        />
                                    </div>
                                    {application.status.sourceHydrator.currentOperation.message && (
                                        <div className='application-status-panel__item-name'>{application.status.sourceHydrator.currentOperation.message}</div>
                                    )}
                                </React.Fragment>
                            )}
                            <div className='application-status-panel__item-name'>
                                {visibleApplication.status?.sourceHydrator?.currentOperation?.drySHA && (
                                    <RevisionMetadataPanel
                                        appName={visibleApplication.metadata.name}
                                        appNamespace={visibleApplication.metadata.namespace}
                                        type={''}
                                        revision={visibleApplication.status.sourceHydrator.currentOperation.drySHA}
                                        versionId={visibleVersionId}
                                    />
                                )}
                            </div>
                        </div>
                    )}
                    <div className='application-status-panel__item'>
                        {sectionHeader(
                            {
                                title: 'SYNC STATUS',
                                helpContent: 'Whether or not the version of your app is up to date with your repo. You may wish to sync your app if it is out-of-sync.'
                            },
                            () => showMetadataInfo && showMetadataInfo(application.status.sync ? 'SYNC_STATUS_REVISION' : null)
                        )}
                        <div
                            className={`application-status-panel__item-value${appOperationState?.phase ? ` application-status-panel__item-value--${appOperationState.phase}` : ''}`}>
                            <div>
                                {application.status.sync.status === models.SyncStatuses.OutOfSync ? (
                                    <a onClick={() => showDiff && showDiff()}>
                                        <ComparisonStatusIcon status={application.status.sync.status} label={true} isButton={true} />
                                    </a>
                                ) : (
                                    <ComparisonStatusIcon status={application.status.sync.status} label={true} />
                                )}
                            </div>
                            <div className='application-status-panel__item-value__revision show-for-large'>{renderSyncStatusRevision(application)}</div>
                        </div>
                        <div className='application-status-panel__item-name' style={{marginBottom: '0.5em'}}>
                            {application.spec.syncPolicy?.automated && application.spec.syncPolicy.automated.enabled !== false
                                ? 'Auto sync is enabled.'
                                : 'Auto sync is not enabled.'}
                        </div>
                        {visibleApplication.status &&
                            visibleApplication.status.sync &&
                            (visibleHasMultipleSources
                                ? visibleApplication.status.sync.revisions &&
                                  visibleApplication.status.sync.revisions[0] &&
                                  visibleApplication.spec.sources &&
                                  !visibleApplication.spec.sources[0].chart
                                : visibleApplication.status.sync.revision && !visibleApplication.spec?.source?.chart) && (
                                <div className='application-status-panel__item-name'>
                                    <RevisionMetadataPanel
                                        appName={visibleApplication.metadata.name}
                                        appNamespace={visibleApplication.metadata.namespace}
                                        type={revisionType}
                                        revision={visibleRevision}
                                        versionId={visibleVersionId}
                                    />
                                </div>
                            )}
                    </div>
                    {visibleAppOperationState && (
                        <div className='application-status-panel__item'>
                            {appOperationState && (
                                <React.Fragment>
                                    {sectionHeader(
                                        {
                                            title: 'LAST SYNC',
                                            helpContent:
                                                'Whether or not your last app sync was successful. It has been ' +
                                                daysSinceLastSynchronized +
                                                ' days since last sync. Click for the status of that sync.'
                                        },
                                        () =>
                                            showMetadataInfo &&
                                            showMetadataInfo(
                                                appOperationState.syncResult && (appOperationState.syncResult.revisions || appOperationState.syncResult.revision)
                                                    ? 'OPERATION_STATE_REVISION'
                                                    : null
                                            )
                                    )}
                                    <div className={`application-status-panel__item-value application-status-panel__item-value--${appOperationState.phase}`}>
                                        {application.status.operationState ? (
                                            <a onClick={() => showOperation && showOperation()}>
                                                <OperationState app={application} isButton={true} />{' '}
                                            </a>
                                        ) : (
                                            // No operation to open; render non-clickable. <span> keeps the icon/label aligned.
                                            <span>
                                                <OperationState app={application} />{' '}
                                            </span>
                                        )}
                                        {appOperationState.syncResult && (appOperationState.syncResult.revision || appOperationState.syncResult.revisions) && (
                                            <div className='application-status-panel__item-value__revision show-for-large'>
                                                to <Revision repoUrl={source.repoURL} revision={operationStateRevision} /> {getAppDefaultSyncRevisionExtra(application)}
                                            </div>
                                        )}
                                    </div>
                                    <div className='application-status-panel__item-name' style={{marginBottom: '0.5em'}}>
                                        {appOperationState.phase} <Timestamp date={appOperationState.finishedAt || appOperationState.startedAt} />
                                    </div>
                                </React.Fragment>
                            )}
                            {(visibleAppOperationState.syncResult && visibleOperationStateRevision && (
                                <RevisionMetadataPanel
                                    appName={visibleApplication.metadata.name}
                                    appNamespace={visibleApplication.metadata.namespace}
                                    type={revisionType}
                                    revision={visibleOperationStateRevision}
                                    versionId={visibleVersionId}
                                />
                            )) ||
                                (appOperationState && <div className='application-status-panel__item-name'>{appOperationState.message}</div>)}
                        </div>
                    )}
                    {application.status.conditions && (
                        <div className={`application-status-panel__item`}>
                            {sectionHeader({title: 'APP CONDITIONS'})}
                            <div className='application-status-panel__item-value application-status-panel__conditions' onClick={() => showConditions && showConditions()}>
                                <ConditionCounters infos={infos} warnings={warnings} errors={errors} />
                            </div>
                        </div>
                    )}
                    <DataLoader
                        key={`${visibleApplication.metadata.namespace}/${visibleApplication.metadata.name}/${visibleApplication.spec.project}`}
                        loadingRenderer={() => null}
                        noLoaderOnInputChange={true}
                        input={{application: visibleApplication, expandCount}}
                        load={async input => {
                            return await services.applications.getApplicationSyncWindowState(input.application.metadata.name, input.application.metadata.namespace);
                        }}>
                        {(data: models.ApplicationSyncWindowState) => (
                            <React.Fragment>
                                {data?.assignedWindows && (
                                    <div className='application-status-panel__item' style={{position: 'relative'}}>
                                        {sectionLabel({
                                            title: 'SYNC WINDOWS',
                                            helpContent:
                                                'The aggregate state of sync windows for this app. ' +
                                                'Red: no syncs allowed. ' +
                                                'Yellow: manual syncs allowed. ' +
                                                'Green: all syncs allowed'
                                        })}
                                        <div className='application-status-panel__item-value'>
                                            <ApplicationSyncWindowStatusIcon project={application.spec.project} state={data} />
                                        </div>
                                    </div>
                                )}
                            </React.Fragment>
                        )}
                    </DataLoader>
                    {showProgressiveSync && <ProgressiveSyncStatus application={visibleApplication} />}
                    {statusExtensions &&
                        statusExtensions.map(ext => <ext.component key={ext.title} application={visibleApplication} openFlyout={() => showExtension && showExtension(ext.id)} />)}
                </div>
            )}
        </React.Fragment>
    );
};
