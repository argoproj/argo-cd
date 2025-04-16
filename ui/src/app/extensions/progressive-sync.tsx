import * as React from 'react';
import {Application, ApplicationTree} from '../shared/models';
import {HelpIcon} from 'argo-ui';
import {ARGO_GRAY6_COLOR} from '../shared/components';
import './progressive-sync.scss';

declare global {
    interface Window {
        extensionsAPI: {
            registerStatusPanelExtension: (component: React.ComponentType<any>, title: string, key: string, flyout?: React.ComponentType<any>) => void;
        };
    }
}

const sectionLabel = (title: string, helpContent?: string) =>
    React.createElement(
        'div',
        {style: {lineHeight: '19.5px', marginBottom: '0.3em'}},
        React.createElement('label', {style: {fontSize: '12px', fontWeight: 600, color: ARGO_GRAY6_COLOR}}, [
            title,
            helpContent && React.createElement(HelpIcon, {title: helpContent})
        ])
    );

const ProgressiveSyncPanel = (props: {application: Application; openFlyout: () => any}) => {
    const isRollingSync = props.application.spec.syncPolicy?.syncOptions?.includes('RollingSync=true');
    const isRunning = props.application.status.operationState?.phase === 'Running';
    const isSynced = props.application.status.sync.status === 'Synced';

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

    return React.createElement('div', {className: 'application-status-panel__item'}, [
        sectionLabel('PROGRESSIVE SYNC', 'Shows if the application is currently in a rolling sync state, waiting for other applications to sync.'),
        React.createElement(
            'div',
            {className: 'application-status-panel__item-value'},
            React.createElement(
                'div',
                {
                    className: 'progressive-sync-status',
                    onClick: () => props.openFlyout(),
                    style: {cursor: 'pointer'}
                },
                [
                    React.createElement('i', {
                        className: `fa ${status.icon}`,
                        style: {color: status.color}
                    }),
                    React.createElement(
                        'span',
                        {
                            style: {
                                marginLeft: '5px',
                                color: status.color
                            }
                        },
                        status.text
                    )
                ]
            )
        )
    ]);
};

const ProgressiveSyncFlyout = (props: {application: Application; tree: ApplicationTree}) => {
    const syncPolicy = props.application.spec.syncPolicy || {};
    const syncOptions = syncPolicy.syncOptions || [];
    const currentWave = props.application.status.operationState?.phase || 'Unknown';
    const syncedResources = props.application.status.resources?.filter(r => r.status === 'Synced') || [];
    const pendingResources = props.application.status.resources?.filter(r => r.status !== 'Synced') || [];

    return React.createElement('div', {style: {padding: '1em'}}, [
        React.createElement('h4', null, 'Progressive Sync Status'),
        React.createElement('div', {style: {marginBottom: '1em'}}, [
            React.createElement('div', null, [React.createElement('strong', null, 'Current Phase: '), currentWave]),
            React.createElement('div', null, [
                React.createElement('strong', null, 'Sync Options: '),
                React.createElement(
                    'ul',
                    null,
                    syncOptions.map((option, i) => React.createElement('li', {key: i}, option))
                )
            ])
        ]),
        React.createElement('div', null, [
            React.createElement('h5', null, 'Resource Status'),
            React.createElement('div', null, [React.createElement('strong', null, 'Synced: '), `${syncedResources.length} resources`]),
            React.createElement('div', null, [React.createElement('strong', null, 'Pending: '), `${pendingResources.length} resources`])
        ]),
        pendingResources.length > 0 &&
            React.createElement('div', null, [
                React.createElement('h5', null, 'Pending Resources'),
                React.createElement(
                    'ul',
                    null,
                    pendingResources.map((resource, i) => React.createElement('li', {key: i}, `${resource.kind}/${resource.name}`))
                )
            ])
    ]);
};

(window => {
    if (window.extensionsAPI) {
        window.extensionsAPI.registerStatusPanelExtension(ProgressiveSyncPanel, 'Progressive Sync', 'progressive_sync', ProgressiveSyncFlyout);
    }
})(window);
