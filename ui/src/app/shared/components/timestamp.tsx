import * as React from 'react';
import Moment from 'react-moment';

// ISO 8601 style, in the browser's local time, so dates read the same in every locale.
export const TIMESTAMP_FORMAT = 'YYYY-MM-DD HH:mm:ss';

export const Timestamp = ({date}: {date: string | number}) => {
    return (
        <span>
            <Moment fromNow={true}>{date}</Moment>
            <span className='show-for-large'>
                {' '}
                (
                <Moment local={true} format={TIMESTAMP_FORMAT}>
                    {date}
                </Moment>
                )
            </span>
        </span>
    );
};
