import * as moment from 'moment';

import {configureMomentRelativeTime} from './timestamp';

test('configureMomentRelativeTime keeps minute granularity until 59 minutes', () => {
    // Moment defaults to switching at 45 minutes ("an hour").
    moment.relativeTimeThreshold('m', 45);
    expect(moment().subtract(50, 'minutes').fromNow()).toMatch(/an hour/);

    configureMomentRelativeTime();

    expect(moment.relativeTimeThreshold('m')).toBe(60);
    expect(moment().subtract(50, 'minutes').fromNow()).toBe('50 minutes ago');
    expect(moment().subtract(59, 'minutes').fromNow()).toBe('59 minutes ago');
    expect(moment().subtract(60, 'minutes').fromNow()).toMatch(/an hour/);
});
