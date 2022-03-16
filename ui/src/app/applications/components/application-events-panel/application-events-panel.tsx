import {DataLoader} from 'argo-ui';
import * as React from 'react';
import {EventsList, ObservableQuery} from '../../../shared/components';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {Event} from '../../../shared/models';

export const ApplicationEventsPanel = ({app}: {app: models.Application}) => {
    return (
        <div className='application-resource-events'>
            <ObservableQuery>
                {q => (
                    <DataLoader loadingRenderer={() => <p>No events yet.</p>} load={() => services.applications.watchEvents(app.metadata.name)}>
                        {(allEvents: Event[]) => <EventsList events={allEvents} />}
                    </DataLoader>
                )}
            </ObservableQuery>
        </div>
    );
};
