import {MockupList} from 'argo-ui';
import * as React from 'react';

import {DataLoader, EventsList} from '../../../shared/components';
import {services} from '../../../shared/services';

export const ApplicationResourceEvents = (props: {applicationName: string; resource?: {namespace: string; name: string; uid: string}}) => (
    <div className='application-resource-events'>
        <DataLoader
            load={() => (props.resource ? services.applications.resourceEvents(props.applicationName, props.resource) : services.applications.events(props.applicationName))}
            loadingRenderer={() => <MockupList height={50} marginTop={10} />}>
            {events => <EventsList events={events} />}
        </DataLoader>
    </div>
);
