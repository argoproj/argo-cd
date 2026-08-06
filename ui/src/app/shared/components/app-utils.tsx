import {Tooltip} from 'argo-ui';
import * as React from 'react';
import {COLORS} from './colors';
import * as appModels from '../models';

require('../../applications/components/utils.scss');

export function helpTip(text: string) {
    return (
        <Tooltip content={text}>
            <span style={{fontSize: 'smaller'}}>
                {' '}
                <i className='fas fa-info-circle' />
            </span>
        </Tooltip>
    );
}

//CLassic Solid circle-notch icon
//<!--!Font Awesome Free 6.7.2 by @fontawesome - https://fontawesome.com License - https://fontawesome.com/license/free Copyright 2025 Fonticons, Inc.-->
//this will replace all <i> fa-spin </i> icons as they are currently misbehaving with no fix available.

export const SpinningIcon = ({color, qeId}: {color: string; qeId: string}) => {
    return (
        <svg className='icon spin' xmlns='http://www.w3.org/2000/svg' viewBox='0 0 512 512' style={{color}} qe-id={qeId}>
            <path
                fill={color}
                d='M222.7 32.1c5 16.9-4.6 34.8-21.5 39.8C121.8 95.6 64 169.1 64 256c0 106 86 192 192 192s192-86 192-192c0-86.9-57.8-160.4-137.1-184.1c-16.9-5-26.6-22.9-21.5-39.8s22.9-26.6 39.8-21.5C434.9 42.1 512 140 512 256c0 141.4-114.6 256-256 256S0 397.4 0 256C0 140 77.1 42.1 182.9 10.6c16.9-5 34.8 4.6 39.8 21.5z'
            />
        </svg>
    );
};

export const getAppOperationState = (app: appModels.Application): appModels.OperationState => {
    if (app.operation) {
        return {
            phase: appModels.OperationPhases.Running,
            message: (app.status && app.status.operationState && app.status.operationState.message) || 'waiting to start',
            startedAt: new Date().toISOString(),
            operation: {
                sync: {}
            }
        } as appModels.OperationState;
    } else if (app.metadata.deletionTimestamp) {
        return {
            phase: appModels.OperationPhases.Running,
            startedAt: app.metadata.deletionTimestamp
        } as appModels.OperationState;
    } else {
        return app.status.operationState;
    }
};

export function getOperationType(application: appModels.Application) {
    const operation = application.operation || (application.status && application.status.operationState && application.status.operationState.operation);
    if (application.metadata.deletionTimestamp && !application.operation) {
        return 'Delete';
    }
    if (operation && operation.sync) {
        return 'Sync';
    }
    return 'Unknown';
}

export const getOperationStateTitle = (app: appModels.Application): appModels.OperationStateTitle => {
    const appOperationState = getAppOperationState(app);
    const operationType = getOperationType(app);
    switch (operationType) {
        case 'Delete':
            return 'Deleting';
        case 'Sync':
            switch (appOperationState.phase) {
                case 'Running':
                    return 'Syncing';
                case 'Error':
                    return 'Sync error';
                case 'Failed':
                    return 'Sync failed';
                case 'Succeeded':
                    return 'Sync OK';
                case 'Terminating':
                    return 'Terminated';
            }
    }
    return 'Unknown';
};

export const OperationPhaseIcon = ({app, isButton}: {app: appModels.Application; isButton?: boolean}) => {
    const operationState = getAppOperationState(app);
    if (operationState === undefined) {
        return null;
    }
    let className = '';
    let color = '';
    switch (operationState.phase) {
        case appModels.OperationPhases.Succeeded:
            className = `fa fa-check-circle${isButton ? ' status-button' : ''}`;
            color = COLORS.operation.success;
            break;
        case appModels.OperationPhases.Error:
            className = `fa fa-times-circle${isButton ? ' status-button' : ''}`;
            color = COLORS.operation.error;
            break;
        case appModels.OperationPhases.Failed:
            className = `fa fa-times-circle${isButton ? ' status-button' : ''}`;
            color = COLORS.operation.failed;
            break;
        default:
            className = 'fa fa-circle-notch fa-spin';
            color = COLORS.operation.running;
            break;
    }
    return className.includes('fa-spin') ? (
        <SpinningIcon color={color} qeId='utils-operations-status-title' />
    ) : (
        <i title={getOperationStateTitle(app)} qe-id='utils-operations-status-title' className={className} style={{color}} />
    );
};

export const ComparisonStatusIcon = ({
    status,
    resource,
    label,
    noSpin,
    isButton
}: {
    status: appModels.SyncStatusCode;
    resource?: {requiresPruning?: boolean};
    label?: boolean;
    noSpin?: boolean;
    isButton?: boolean;
}) => {
    let className = 'fas fa-question-circle';
    let color = COLORS.sync.unknown;
    let title: string = 'Unknown';
    switch (status) {
        case appModels.SyncStatuses.Synced:
            className = `fa fa-check-circle${isButton ? ' status-button' : ''}`;
            color = COLORS.sync.synced;
            title = 'Synced';
            break;
        case appModels.SyncStatuses.OutOfSync:
            // eslint-disable-next-line no-case-declarations
            const requiresPruning = resource && resource.requiresPruning;
            className = requiresPruning ? `fa fa-trash${isButton ? ' status-button' : ''}` : `fa fa-arrow-alt-circle-up${isButton ? ' status-button' : ''}`;
            title = 'OutOfSync';
            if (requiresPruning) {
                title = `${title} (This resource is not present in the application's source. It will be deleted from Kubernetes if the prune option is enabled during sync.)`;
            }
            color = COLORS.sync.out_of_sync;
            break;
        case appModels.SyncStatuses.Unknown:
            className = `fa fa-circle-notch ${noSpin ? '' : 'fa-spin'}${isButton ? ' status-button' : ''}`;
            break;
    }
    return className.includes('fa-spin') ? (
        <SpinningIcon color={color} qeId='utils-sync-status-title' />
    ) : (
        <React.Fragment>
            <i qe-id='utils-sync-status-title' title={title} className={className} style={{color}} /> {label && title}
        </React.Fragment>
    );
};

export const HealthStatusIcon = ({state, noSpin}: {state: appModels.HealthStatus; noSpin?: boolean}) => {
    let color = COLORS.health.unknown;
    let icon = 'fa-question-circle';

    switch (state.status) {
        case appModels.HealthStatuses.Healthy:
            color = COLORS.health.healthy;
            icon = 'fa-heart';
            break;
        case appModels.HealthStatuses.Suspended:
            color = COLORS.health.suspended;
            icon = 'fa-pause-circle';
            break;
        case appModels.HealthStatuses.Degraded:
            color = COLORS.health.degraded;
            icon = 'fa-heart-broken';
            break;
        case appModels.HealthStatuses.Progressing:
            color = COLORS.health.progressing;
            icon = `fa fa-circle-notch ${noSpin ? '' : 'fa-spin'}`;
            break;
        case appModels.HealthStatuses.Missing:
            color = COLORS.health.missing;
            icon = 'fa-ghost';
            break;
    }
    let title: string = state.status;
    if (state.message) {
        title = `${state.status}: ${state.message}`;
    }
    return icon.includes('fa-spin') ? (
        <SpinningIcon color={color} qeId='utils-health-status-title' />
    ) : (
        <i qe-id='utils-health-status-title' title={title} className={'fa ' + icon + ' utils-health-status-icon'} style={{color}} />
    );
};

export const SyncWindowStatusIcon = ({state, window}: {state: appModels.SyncWindowsState; window: appModels.SyncWindow}) => {
    let className = '';
    let color = '';
    let current = '';

    if (state.windows === undefined) {
        current = 'Inactive';
    } else {
        for (const w of state.windows) {
            if (w.kind === window.kind && w.schedule === window.schedule && w.duration === window.duration && w.timeZone === window.timeZone) {
                current = 'Active';
                break;
            } else {
                current = 'Inactive';
            }
        }
    }

    switch (current + ':' + window.kind) {
        case 'Active:deny':
        case 'Inactive:allow':
            className = 'fa fa-stop-circle';
            if (window.manualSync) {
                color = COLORS.sync_window.manual;
            } else {
                color = COLORS.sync_window.deny;
            }
            break;
        case 'Active:allow':
        case 'Inactive:deny':
            className = 'fa fa-check-circle';
            color = COLORS.sync_window.allow;
            break;
        default:
            className = 'fas fa-question-circle';
            color = COLORS.sync_window.unknown;
            current = 'Unknown';
            break;
    }

    return (
        <React.Fragment>
            <i title={current} className={className} style={{color}} /> {current}
        </React.Fragment>
    );
};

export function isApp(abstractApp: appModels.AbstractApplication): abstractApp is appModels.Application {
    return abstractApp.kind === 'Application';
}

export function getRootPathByApp(abstractApp: appModels.AbstractApplication) {
    return isApp(abstractApp) ? '/applications' : '/applicationsets';
}

export function appQualifiedName(app: appModels.AbstractApplication, nsEnabled: boolean): string {
    return (nsEnabled ? app.metadata.namespace + '/' : '') + app.metadata.name;
}

export function appInstanceName(app: appModels.AbstractApplication): string {
    return app.metadata.namespace + '_' + app.metadata.name;
}
