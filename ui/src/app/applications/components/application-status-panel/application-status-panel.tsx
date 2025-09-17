import {HelpIcon} from 'argo-ui';
import * as React from 'react';
import {ARGO_GRAY6_COLOR, DataLoader} from '../../../shared/components';
import {Revision} from '../../../shared/components/revision';
import {Timestamp} from '../../../shared/components/timestamp';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {
    ApplicationSyncWindowStatusIcon,
    ComparisonStatusIcon,
    getAppDefaultSource,
    getAppDefaultSyncRevisionExtra,
    getAppOperationState,
    HydrateOperationPhaseIcon,
    hydrationStatusMessage,
    getProgressiveSyncStatusColor,
    getProgressiveSyncStatusIcon
} from '../utils';
import {getConditionCategory, HealthStatusIcon, OperationState, syncStatusMessage, getAppDefaultSyncRevision, getAppDefaultOperationSyncRevision} from '../utils';
import {RevisionMetadataPanel} from './revision-metadata-panel';
import * as utils from '../utils';
import {COLORS} from '../../../shared/components/colors';

import './application-status-panel.scss';

interface Props {
    application: models.Application;
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
    <label style={{fontSize: '12px', fontWeight: 600, color: ARGO_GRAY6_COLOR}}>
        {info.title}
        {info.helpContent && <HelpIcon title={info.helpContent} />}
    </label>
);

const sectionHeader = (info: SectionInfo, onClick?: () => any) => {
    return (
        <div style={{display: 'flex', alignItems: 'center', marginBottom: '0.5em'}}>
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
                // Check if user has permission to read ApplicationSets
                const canReadApplicationSets = await services.accounts.canI('applicationsets', 'get', application.spec.project + '/' + application.metadata.name);

                // Find ApplicationSet by searching all namespaces dynamically
                const appSetList = await services.applications.listApplicationSets();
                const appSet = appSetList.items?.find(item => item.metadata.name === appSetRef.name);

                if (!appSet) {
                    throw new Error(`ApplicationSet ${appSetRef.name} not found in any namespace`);
                }

                return {canReadApplicationSets, appSet};
            }}>
            {({canReadApplicationSets, appSet}: {canReadApplicationSets: boolean; appSet: models.ApplicationSet}) => {
                // Hide panel if: Progressive Sync disabled, no permission, or not RollingSync strategy
                if (!appSet.status?.applicationStatus || appSet?.spec?.strategy?.type !== 'RollingSync' || !canReadApplicationSets) {
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
                        {appResource?.step && <div className='application-status-panel__item-value'>Wave: {appResource.step}</div>}
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

export const ApplicationStatusPanel = ({application, showDiff, showOperation, showHydrateOperation, showConditions, showExtension, showMetadataInfo}: Props) => {
    const [showProgressiveSync, setShowProgressiveSync] = React.useState(false);

    React.useEffect(() => {
        // Only show Progressive Sync if the application has an ApplicationSet parent
        // The actual strategy validation will be done inside ProgressiveSyncStatus component
        setShowProgressiveSync(!!getApplicationSetOwnerRef(application));
    }, [application]);

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
    if (application.metadata.deletionTimestamp && !appOperationState) {
        showOperation = null;
    }

    const statusExtensions = services.extensions.getStatusPanelExtensions();

    const revision = getAppDefaultSyncRevision(application);
    const operationStateRevision = getAppDefaultOperationSyncRevision(application);
    const infos = cntByCategory.get('info');
    const warnings = cntByCategory.get('warning');
    const errors = cntByCategory.get('error');
    const source = getAppDefaultSource(application);
    const hasMultipleSources = application.spec.sources?.length > 0;
    const revisionType = source?.repoURL?.startsWith('oci://') ? 'oci' : source?.chart ? 'helm' : 'git';
    return (
        <div className='application-status-panel row'>
            <div className='application-status-panel__item'>
                <div style={{lineHeight: '19.5px', marginBottom: '0.3em'}}>{sectionLabel({title: 'APP HEALTH', helpContent: 'The health status of your app'})}</div>
                <div className='application-status-panel__item-value'>
                    <HealthStatusIcon state={application.status.health} />
                    &nbsp;
                    {application.status.health.status}
                </div>
                {application.status.health.message && <div className='application-status-panel__item-name'>{application.status.health.message}</div>}
            </div>
            {application.spec.sourceHydrator && application.status?.sourceHydrator?.currentOperation && (
                <div className='application-status-panel__item'>
                    <div style={{lineHeight: '19.5px', marginBottom: '0.3em'}}>
                        {sectionLabel({
                            title: 'SOURCE HYDRATOR',
                            helpContent: 'The source hydrator reads manifests from git, hydrates (renders) them, and pushes them to a different location in git.'
                        })}
                    </div>
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
                        <Timestamp date={application.status.sourceHydrator.currentOperation.finishedAt || application.status.sourceHydrator.currentOperation.startedAt} />
                    </div>
                    {application.status.sourceHydrator.currentOperation.message && (
                        <div className='application-status-panel__item-name'>{application.status.sourceHydrator.currentOperation.message}</div>
                    )}
                    <div className='application-status-panel__item-name'>
                        {application.status.sourceHydrator.currentOperation.drySHA && (
                            <RevisionMetadataPanel
                                appName={application.metadata.name}
                                appNamespace={application.metadata.namespace}
                                type={''}
                                revision={application.status.sourceHydrator.currentOperation.drySHA}
                                versionId={utils.getAppCurrentVersion(application)}
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
                    () => showMetadataInfo(application.status.sync ? 'SYNC_STATUS_REVISION' : null)
                )}
                <div className={`application-status-panel__item-value${appOperationState?.phase ? ` application-status-panel__item-value--${appOperationState.phase}` : ''}`}>
                    <div>
                        {application.status.sync.status === models.SyncStatuses.OutOfSync ? (
                            <a onClick={() => showDiff && showDiff()}>
                                <ComparisonStatusIcon status={application.status.sync.status} label={true} isButton={true} />
                            </a>
                        ) : (
                            <ComparisonStatusIcon status={application.status.sync.status} label={true} />
                        )}
                    </div>
                    <div className='application-status-panel__item-value__revision show-for-large'>{syncStatusMessage(application)}</div>
                </div>
                <div className='application-status-panel__item-name' style={{marginBottom: '0.5em'}}>
                    {application.spec.syncPolicy?.automated && application.spec.syncPolicy.automated.enabled !== false ? 'Auto sync is enabled.' : 'Auto sync is not enabled.'}
                </div>
                {application.status &&
                    application.status.sync &&
                    (hasMultipleSources
                        ? application.status.sync.revisions && application.status.sync.revisions[0] && application.spec.sources && !application.spec.sources[0].chart
                        : application.status.sync.revision && !application.spec?.source?.chart) && (
                        <div className='application-status-panel__item-name'>
                            <RevisionMetadataPanel
                                appName={application.metadata.name}
                                appNamespace={application.metadata.namespace}
                                type={revisionType}
                                revision={revision}
                                versionId={utils.getAppCurrentVersion(application)}
                            />
                        </div>
                    )}
            </div>
            {appOperationState && (
                <div className='application-status-panel__item'>
                    {sectionHeader(
                        {
                            title: 'LAST SYNC',
                            helpContent:
                                'Whether or not your last app sync was successful. It has been ' +
                                daysSinceLastSynchronized +
                                ' days since last sync. Click for the status of that sync.'
                        },
                        () =>
                            showMetadataInfo(
                                appOperationState.syncResult && (appOperationState.syncResult.revisions || appOperationState.syncResult.revision)
                                    ? 'OPERATION_STATE_REVISION'
                                    : null
                            )
                    )}
                    <div className={`application-status-panel__item-value application-status-panel__item-value--${appOperationState.phase}`}>
                        <a onClick={() => showOperation && showOperation()}>
                            <OperationState app={application} isButton={true} />{' '}
                        </a>
                        {appOperationState.syncResult && (appOperationState.syncResult.revision || appOperationState.syncResult.revisions) && (
                            <div className='application-status-panel__item-value__revision show-for-large'>
                                to <Revision repoUrl={source.repoURL} revision={operationStateRevision} /> {getAppDefaultSyncRevisionExtra(application)}
                            </div>
                        )}
                    </div>
                    <div className='application-status-panel__item-name' style={{marginBottom: '0.5em'}}>
                        {appOperationState.phase} <Timestamp date={appOperationState.finishedAt || appOperationState.startedAt} />
                    </div>
                    {(appOperationState.syncResult && operationStateRevision && (
                        <RevisionMetadataPanel
                            appName={application.metadata.name}
                            appNamespace={application.metadata.namespace}
                            type={revisionType}
                            revision={operationStateRevision}
                            versionId={utils.getAppCurrentVersion(application)}
                        />
                    )) || <div className='application-status-panel__item-name'>{appOperationState.message}</div>}
                </div>
            )}
            {application.status.conditions && (
                <div className={`application-status-panel__item`}>
                    {sectionLabel({title: 'APP CONDITIONS'})}
                    <div className='application-status-panel__item-value application-status-panel__conditions' onClick={() => showConditions && showConditions()}>
                        {infos && (
                            <a className='info'>
                                <i className='fa fa-info-circle application-status-panel__item-value__status-button' /> {infos} Info
                            </a>
                        )}
                        {warnings && (
                            <a className='warning'>
                                <i className='fa fa-exclamation-triangle application-status-panel__item-value__status-button' /> {warnings} Warning{warnings !== 1 && 's'}
                            </a>
                        )}
                        {errors && (
                            <a className='error'>
                                <i className='fa fa-exclamation-circle application-status-panel__item-value__status-button' /> {errors} Error{errors !== 1 && 's'}
                            </a>
                        )}
                    </div>
                </div>
            )}
            <DataLoader
                noLoaderOnInputChange={true}
                input={application}
                load={async app => {
                    return await services.applications.getApplicationSyncWindowState(app.metadata.name, app.metadata.namespace);
                }}>
                {(data: models.ApplicationSyncWindowState) => (
                    <React.Fragment>
                        {data.assignedWindows && (
                            <div className='application-status-panel__item' style={{position: 'relative'}}>
                                {sectionLabel({
                                    title: 'SYNC WINDOWS',
                                    helpContent:
                                        'The aggregate state of sync windows for this app. ' +
                                        'Red: no syncs allowed. ' +
                                        'Yellow: manual syncs allowed. ' +
                                        'Green: all syncs allowed'
                                })}
                                <div className='application-status-panel__item-value' style={{margin: 'auto 0'}}>
                                    <ApplicationSyncWindowStatusIcon project={application.spec.project} state={data} />
                                </div>
                            </div>
                        )}
                    </React.Fragment>
                )}
            </DataLoader>
            {showProgressiveSync && <ProgressiveSyncStatus application={application} />}
            {statusExtensions && statusExtensions.map(ext => <ext.component key={ext.title} application={application} openFlyout={() => showExtension && showExtension(ext.id)} />)}
        </div>
    );
};
