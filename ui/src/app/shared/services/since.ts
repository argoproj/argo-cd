export type Since = 'forever' | '1m ago' | '5m ago' | '30m ago' | '1h ago' | '4h ago';

const sinceToMins = (value: Since) => {
    switch (value) {
        case '1m ago':
            return 1;
        case '5m ago':
            return 5;
        case '30m ago':
            return 30;
        case '1h ago':
            return 60;
        case '4h ago':
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
