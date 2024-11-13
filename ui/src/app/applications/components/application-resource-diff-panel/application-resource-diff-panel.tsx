import {DataLoader, SlidingPanel} from 'argo-ui';
import * as React from 'react';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ApplicationResourcesDiff} from '../application-resources-diff/application-resources-diff';

export const ApplicationResourceDiffPanel = ({application, hide}: {application: models.Application; hide: () => any}) => {
    const isVisible = !!application;
    const header = application?.metadata.name || 'diff';
    return (
        <SlidingPanel header={header} isShown={isVisible} onClose={() => hide()}>
            {isVisible && (
                <DataLoader
                    key='diff'
                    load={async () =>
                        await services.applications.managedResources(application.metadata.name, application.metadata.namespace, {
                            fields: ['items.normalizedLiveState', 'items.predictedLiveState', 'items.group', 'items.kind', 'items.namespace', 'items.name']
                        })
                    }>
                    {managedResources => <ApplicationResourcesDiff states={managedResources} />}
                </DataLoader>
            )}
        </SlidingPanel>
    );
};
