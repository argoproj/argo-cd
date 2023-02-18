export type Since = 'min' | '1m' | '5m' | '30m' | '1h' | '4h';

const sinceToMins = (value: Since) => {
    switch (value) {
        case '1m':
            return 1;
        case '5m':
            return 5;
        case '30m':
            return 30;
        case '1h':
            return 60;
        case '4h':
            return 4 * 60;
        default:
            return 0;
    }
};

export const sinceSeconds = (since: Since, now = new Date()): number => {
    const mins = sinceToMins(since);
    const date = mins === 0 ? new Date(0) : new Date(now.getTime() - mins * 60 * 1000);
    return date.getTime() / 1000;
};
