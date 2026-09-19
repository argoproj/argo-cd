import * as React from 'react';
import Moment from 'react-moment';
import * as moment from 'moment';

// Moment's default minute threshold is 45, so relative ages jump from ~44m to "an hour".
// Raise it to 60 so UI ages count through 59 minutes before switching to hours (#29776).
export function configureMomentRelativeTime(): void {
    moment.relativeTimeThreshold('m', 60);
}

configureMomentRelativeTime();

export const Timestamp = ({date}: {date: string | number}) => {
    return (
        <span>
            <Moment fromNow={true}>{date}</Moment>
            <span className='show-for-large'>
                {' '}
                (<Moment local={true}>{date}</Moment>)
            </span>
        </span>
    );
};
