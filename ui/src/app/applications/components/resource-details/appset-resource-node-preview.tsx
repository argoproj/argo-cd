import * as React from 'react';

import * as models from '../../../shared/models';
import {AppSetGeneratedAppsDiff} from './appset-generated-apps-diff';
import './resource-details.scss';

interface AppSetResourceNodePreviewProps {
    liveAppSet: any;
    targetAppSet: any;
    syncStatus: models.SyncStatusCode;
}

function toAppSet(raw: any, fallbackMeta?: {name?: string; namespace?: string}): models.ApplicationSet {
    if (raw && typeof raw === 'object' && raw.metadata && raw.spec) {
        return raw as models.ApplicationSet;
    }
    return {
        apiVersion: 'argoproj.io/v1alpha1',
        kind: 'ApplicationSet',
        metadata: {
            name: fallbackMeta?.name || '',
            namespace: fallbackMeta?.namespace || ''
        },
        spec: {}
    } as unknown as models.ApplicationSet;
}

export const AppSetResourceNodePreview = (props: AppSetResourceNodePreviewProps) => {
    const {liveAppSet, targetAppSet, syncStatus} = props;

    if (syncStatus === models.SyncStatuses.Synced) {
        return (
            <div className='applicationset-preview' style={{padding: '15px'}}>
                <div className='white-box'>
                    <div className='white-box__details'>
                        <i className='fa fa-check-circle appset-resource-node-preview__synced-icon' />
                        ApplicationSet is in sync — no preview available.
                    </div>
                </div>
            </div>
        );
    }

    if (!targetAppSet) {
        return (
            <div className='applicationset-preview' style={{padding: '15px'}}>
                <div className='white-box'>
                    <div className='white-box__details'>No target ApplicationSet manifest available to preview.</div>
                </div>
            </div>
        );
    }

    const fallbackMeta = {
        name: targetAppSet?.metadata?.name,
        namespace: targetAppSet?.metadata?.namespace
    };
    const currentAppSet = toAppSet(liveAppSet, fallbackMeta);
    const proposedAppSet = toAppSet(targetAppSet, fallbackMeta);
    const project = (proposedAppSet.spec as any)?.template?.spec?.project || 'unknown';

    return (
        <div className='applicationset-preview' style={{padding: '15px'}}>
            <div style={{marginBottom: '15px', fontSize: '13px', color: '#6d7f8b'}}>
                <i className='fa fa-info-circle' style={{marginRight: '6px'}} />
                Preview shows what child Applications will change when this app-of-appset syncs.
            </div>
            <AppSetGeneratedAppsDiff currentAppSet={currentAppSet} proposedAppSet={proposedAppSet} trigger={1} rbacProject={project} />
        </div>
    );
};
