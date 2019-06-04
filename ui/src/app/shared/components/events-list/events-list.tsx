import * as React from 'react';

import * as models from '../../models';

require('./events-list.scss');

export const EventsList = (props: { events: models.Event[] }) => (
    <div className='events-list'>
        {props.events.length === 0 && (
            <p>No events available</p>
        ) || (
            <div className='argo-table-list'>
                <div className='argo-table-list__head'>
                    <div className='row'>
                        <div className='columns small-2'>REASON</div>
                        <div className='columns small-6'>MESSAGE</div>
                        <div className='columns small-2'>COUNT</div>
                        <div className='columns small-2'>EVENT TIMESTAMP</div>
                    </div>
                </div>
                {props.events.map((event) => (
                    <div className={`argo-table-list__row events-list__event events-list__event--${event.type}`}
                            key={event.metadata.uid}>
                        <div className='row'>
                            <div className='columns small-2'>{event.reason}</div>
                            <div className='columns small-6'>{event.message}</div>
                            <div className='columns small-2'>{event.count}</div>
                            <div className='columns small-2'>{event.firstTimestamp}</div>
                        </div>
                    </div>
                ))}
            </div>
        )}
    </div>
);
