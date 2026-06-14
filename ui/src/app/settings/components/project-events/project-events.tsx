import {MockupList} from 'argo-ui';
import * as React from 'react';

import {DataLoader, EventsList} from '../../../shared/components';
import {services} from '../../../shared/services';

export const ProjectEvents = (props: {projectName: string}) => (
    <div className='application-resource-events'>
        <DataLoader load={() => services.projects.events(props.projectName)} loadingRenderer={() => <MockupList height={50} marginTop={10} />}>
            {events => <EventsList events={events} />}
        </DataLoader>
    </div>
);
