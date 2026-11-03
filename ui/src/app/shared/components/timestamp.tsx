import * as React from 'react';
import Moment from 'react-moment';

const TIMESTAMP_FORMAT_OPTIONS: Intl.DateTimeFormatOptions = {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    // The rest of the UI is English, so keep 0-9 digits even for locales that default to another
    // numbering system (e.g. Devanagari for mr-IN).
    numberingSystem: 'latn'
};

// Formats a date in the browser's locale and local time, e.g. 09/03/2026, 12:00:00 PM for en-US
// and 03/09/2026, 12:00:00 for en-GB, so the day/month order matches what the user expects.
export function formatTimestamp(date: string | number | Date, locale?: string | string[]): string {
    return new Date(date).toLocaleString(locale, TIMESTAMP_FORMAT_OPTIONS);
}

export const Timestamp = ({date}: {date: string | number}) => {
    return (
        <span>
            <Moment fromNow={true}>{date}</Moment>
            <span className='show-for-large'> ({formatTimestamp(date)})</span>
        </span>
    );
};
