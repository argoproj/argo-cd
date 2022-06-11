import * as React from 'react';
import Moment from 'react-moment';

export const Timestamp = ({date}: {date: string | number}) => {
    return (
        <span>
            <Moment fromNow={true}>{date}</Moment> (<Moment local={true}>{date}</Moment>)
        </span>
    );
};
