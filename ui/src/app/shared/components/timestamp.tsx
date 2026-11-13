import * as React from 'react';
import Moment from 'react-moment';

const TIMESTAMP_FORMAT_OPTIONS: Intl.DateTimeFormatOptions = {
    year: 'numeric',
    month: 'short',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    // Keep 0-9 digits even for regions that default to another numbering system.
    numberingSystem: 'latn'
};

// The UI is English-only, so format dates in English using the region of the user's locale: the
// day/month order and 12/24-hour clock follow the region, and month names stay English (e.g. mr-IN
// is formatted as en-IN). Without a region, plain English is used.
export function englishLocaleFor(locale?: string): string {
    const requested = locale ?? (typeof navigator !== 'undefined' ? navigator.language : undefined);
    try {
        const region = requested ? new Intl.Locale(requested).region : undefined;
        return region ? `en-${region}` : 'en';
    } catch {
        return 'en';
    }
}

// Formats a date in local time, e.g. Sep 03, 2026, 12:00:00 PM for en-US and 03 Sept 2026, 12:00:00
// for en-GB. The textual month keeps the date unambiguous when it is shared between users.
export function formatTimestamp(date: string | number | Date, locale?: string): string {
    return new Date(date).toLocaleString(englishLocaleFor(locale), TIMESTAMP_FORMAT_OPTIONS);
}

// Date-only variant for narrow columns, e.g. Sep 03, 2026 for en-US and 03 Sept 2026 for en-GB.
export function formatDate(date: string | number | Date, locale?: string): string {
    const {year, month, day, numberingSystem} = TIMESTAMP_FORMAT_OPTIONS;
    return new Date(date).toLocaleDateString(englishLocaleFor(locale), {year, month, day, numberingSystem});
}

export const Timestamp = ({date}: {date: string | number}) => {
    return (
        <span>
            <Moment fromNow={true}>{date}</Moment>
            <span className='show-for-large'> ({formatTimestamp(date)})</span>
        </span>
    );
};
