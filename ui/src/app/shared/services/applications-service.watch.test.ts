import {Observable} from 'rxjs';

jest.mock('./requests', () => ({
    __esModule: true,
    default: {loadEventSource: jest.fn()}
}));

jest.mock('../../applications/components/utils', () => ({
    getRootPathByApp: jest.fn(),
    isApp: jest.fn()
}));

import requests from './requests';
import {ApplicationsService} from './applications-service';

describe('ApplicationsService.watch reconnects', () => {
    beforeEach(() => {
        jest.useFakeTimers();
    });

    afterEach(() => {
        jest.useRealTimers();
    });

    it('backs off between reconnect attempts instead of retrying immediately', () => {
        const counter = {subscriptions: 0};
        (requests.loadEventSource as jest.Mock).mockImplementation(
            () =>
                new Observable<string>(subscriber => {
                    counter.subscriptions++;
                    const failure = setTimeout(() => subscriber.error(new Error('boom')), 1);
                    return () => clearTimeout(failure);
                })
        );
        const subscription = new ApplicationsService().watch('application').subscribe({error: () => undefined});
        expect(counter.subscriptions).toBe(1);
        jest.advanceTimersByTime(1);
        expect(counter.subscriptions).toBe(1);
        jest.advanceTimersByTime(500);
        expect(counter.subscriptions).toBe(2);
        subscription.unsubscribe();
    });
});
