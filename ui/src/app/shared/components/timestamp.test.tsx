import * as moment from 'moment';
import * as React from 'react';
import {renderToStaticMarkup} from 'react-dom/server.node';
import {Timestamp} from './timestamp';

test('Timestamp shows the absolute date as year-month-day', () => {
    const date = '2026-09-03T12:00:00Z';
    const html = renderToStaticMarkup(<Timestamp date={date} />);
    // The date is rendered in local time, so compute the expected value in the test's timezone.
    expect(html).toContain(moment(date).format('YYYY-MM-DD HH:mm:ss'));
    expect(html).not.toContain('GMT');
});
