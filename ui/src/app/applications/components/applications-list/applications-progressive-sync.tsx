import * as React from 'react';
import {Application} from '../../../shared/models';
import {Tooltip} from 'argo-ui';

export const ApplicationsProgressiveSync = ({app}: {app: Application}) => {
    const isRollingSync = app.spec.syncPolicy?.syncOptions?.includes('RollingSync=true');
    const isRunning = app.status.operationState?.phase === 'Running';
    const isSynced = app.status.sync.status === 'Synced';

    if (!isRollingSync) {
        return null;
    }

    const getStatusIcon = () => {
        if (isRunning) {
            return {
                icon: 'fa-sync-alt rotating',
                color: '#0DADEA',
                text: 'RollingSync in Progress'
            };
        } else if (isSynced) {
            return {
                icon: 'fa-sync-alt',
                color: '#18BE94',
                text: 'RollingSync Complete'
            };
        }
        return {
            icon: 'fa-sync-alt',
            color: '#CCD6DD',
            text: 'RollingSync Enabled'
        };
    };

    const status = getStatusIcon();

    return (
        <div className='applications-list__progressive-sync'>
            <Tooltip content={status.text}>
                <div>
                    <i className={`fa ${status.icon}`} style={{color: status.color}} />
                    <span style={{marginLeft: '5px', color: status.color}}>{status.text}</span>
                </div>
            </Tooltip>
        </div>
    );
}; 