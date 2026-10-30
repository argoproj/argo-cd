import {useQuery} from '@tanstack/react-query';
import {MockupList} from 'argo-ui';
import * as React from 'react';

import {EventsList} from '../../../shared/components';
import {projectQueries} from '../../../shared/queries';
import {loadFailed} from '../../../shared/query-client';

export const ProjectEvents = (props: {projectName: string}) => {
    const {data: events, isError, refetch} = useQuery(projectQueries.events(props.projectName));
    return <div className='application-resource-events'>{isError ? loadFailed(refetch) : events ? <EventsList events={events} /> : <MockupList height={50} marginTop={10} />}</div>;
};
