import { MockupList } from 'argo-ui';
import * as React from 'react';

import { DataLoader, EventsList } from '../../../shared/components';
import * as models from '../../../shared/models';
import { services } from '../../../shared/services';

export const ApplicationResourceEvents = (props: { applicationName: string, resource?: models.ResourceNode }) => (
    <div className='application-resource-events'>
        <DataLoader load={() => props.resource ?
            services.applications.resourceEvents(props.applicationName, props.resource.state.metadata.uid, props.resource.state.metadata.name) :
            services.applications.events(props.applicationName)}
            loadingRenderer={() => <MockupList height={50} marginTop={10}/>}>
            {(events) => <EventsList events={events}/>}
        </DataLoader>
    </div>
);
