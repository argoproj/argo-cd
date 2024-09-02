import * as moment from 'moment';
import * as React from 'react';
import {ago} from 'argo-ui/v2';

import * as models from '../../models';

require('./events-list.scss');

function timestampSort(first: models.Event, second: models.Event): number {
    if (first.lastTimestamp && !second.lastTimestamp) {
        return moment(second.eventTime).diff(first.lastTimestamp);
    } else if (!first.lastTimestamp && second.lastTimestamp) {
        return moment(second.lastTimestamp).diff(first.eventTime);
    } else if (!first.lastTimestamp && !second.lastTimestamp) {
        return moment(second.eventTime).diff(first.eventTime);
    }
    return moment(second.lastTimestamp).diff(first.lastTimestamp);
}

function getTimeElements(timestamp: string) {
    const readableFormat = (d: Date) => moment(d).calendar();
    const dateOfEvent = new Date(timestamp);
    return (
        <>
            <div className='events-list__event__ago'>{ago(dateOfEvent)}</div>
            <div className='events-list__event__time'>{readableFormat(dateOfEvent)}</div>
        </>
    );
}

export const EventsList = (props: {events: models.Event[]}) => {
    const events = props.events.sort((first, second) => timestampSort(first, second));

    return (
        <div className='events-list'>
            {(events.length === 0 && <p>No events available</p>) || (
                <div className='argo-table-list'>
                    <div className='argo-table-list__head'>
                        <div className='row'>
                            <div className='columns small-2 xxlarge-2'>REASON</div>
                            <div className='columns small-4 xxlarge-5'>MESSAGE</div>
                            <div className='columns small-2 xxlarge-1'>COUNT</div>
                            <div className='columns small-2 xxlarge-2'>FIRST OCCURRED</div>
                            <div className='columns small-2 xxlarge-2'>LAST OCCURRED</div>
                        </div>
                    </div>
                    {events.map(event => (
                        <div className={`argo-table-list__row events-list__event events-list__event--${event.type}`} key={event.metadata.uid}>
                            <div className='row'>
                                <div className='columns small-2 xxlarge-2'>{event.reason}</div>
                                <div className='columns small-4 xxlarge-5' style={{whiteSpace: 'normal', lineHeight: 'normal'}}>
                                    {event.message}
                                </div>
                                <div className='columns small-2 xxlarge-1'>{event.count}</div>
                                <div className='columns small-2 xxlarge-2'>{event.firstTimestamp ? getTimeElements(event.firstTimestamp) : getTimeElements(event.eventTime)}</div>
                                <div className='columns small-2 xxlarge-2'>{event.lastTimestamp ? getTimeElements(event.lastTimestamp) : getTimeElements(event.eventTime)}</div>
                            </div>
                        </div>
                    ))}
                </div>
            )}
        </div>
    );
};
