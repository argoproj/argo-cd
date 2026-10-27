import {Observable, Subscriber} from 'rxjs';
import {retryWithBackoff} from './retry-backoff';

describe('retryWithBackoff', () => {
    beforeEach(() => {
        jest.useFakeTimers();
    });

    afterEach(() => {
        jest.useRealTimers();
    });

    function alwaysFailingSource() {
        const counter = {subscriptions: 0};
        const source = new Observable<string>(subscriber => {
            counter.subscriptions++;
            subscriber.error(new Error('boom'));
        });
        return {source, counter};
    }

    function manualSource() {
        const subscribers: Subscriber<string>[] = [];
        const source = new Observable<string>(subscriber => {
            subscribers.push(subscriber);
        });
        return {source, subscribers};
    }

    it('passes emitted values through to the subscriber', () => {
        const {source, subscribers} = manualSource();
        const received: string[] = [];
        const subscription = source.pipe(retryWithBackoff()).subscribe(value => received.push(value));
        subscribers[0].next('hello');
        expect(received).toEqual(['hello']);
        subscription.unsubscribe();
    });

    it('retries after the initial delay on the first failure', () => {
        const {source, counter} = alwaysFailingSource();
        const subscription = source.pipe(retryWithBackoff({initialDelayMs: 500, maxDelayMs: 30000})).subscribe({error: () => undefined});
        expect(counter.subscriptions).toBe(1);
        jest.advanceTimersByTime(499);
        expect(counter.subscriptions).toBe(1);
        jest.advanceTimersByTime(1);
        expect(counter.subscriptions).toBe(2);
        subscription.unsubscribe();
    });

    it('doubles the delay after each consecutive failure', () => {
        const {source, counter} = alwaysFailingSource();
        const subscription = source.pipe(retryWithBackoff({initialDelayMs: 500, maxDelayMs: 30000})).subscribe({error: () => undefined});
        jest.advanceTimersByTime(500);
        expect(counter.subscriptions).toBe(2);
        jest.advanceTimersByTime(999);
        expect(counter.subscriptions).toBe(2);
        jest.advanceTimersByTime(1);
        expect(counter.subscriptions).toBe(3);
        jest.advanceTimersByTime(1999);
        expect(counter.subscriptions).toBe(3);
        jest.advanceTimersByTime(1);
        expect(counter.subscriptions).toBe(4);
        subscription.unsubscribe();
    });

    it('caps the delay at maxDelayMs', () => {
        const {source, counter} = alwaysFailingSource();
        const subscription = source.pipe(retryWithBackoff({initialDelayMs: 500, maxDelayMs: 1000})).subscribe({error: () => undefined});
        jest.advanceTimersByTime(500);
        expect(counter.subscriptions).toBe(2);
        jest.advanceTimersByTime(1000);
        expect(counter.subscriptions).toBe(3);
        jest.advanceTimersByTime(1000);
        expect(counter.subscriptions).toBe(4);
        subscription.unsubscribe();
    });

    it('resets to the initial delay after the stream stays healthy for resetAfterMs', () => {
        const {source, subscribers} = manualSource();
        const subscription = source.pipe(retryWithBackoff({initialDelayMs: 500, maxDelayMs: 30000, resetAfterMs: 60000})).subscribe({error: () => undefined});
        subscribers[0].error(new Error('boom'));
        jest.advanceTimersByTime(500);
        expect(subscribers.length).toBe(2);
        subscribers[1].error(new Error('boom'));
        jest.advanceTimersByTime(1000);
        expect(subscribers.length).toBe(3);
        jest.advanceTimersByTime(60000);
        subscribers[2].error(new Error('boom'));
        jest.advanceTimersByTime(499);
        expect(subscribers.length).toBe(3);
        jest.advanceTimersByTime(1);
        expect(subscribers.length).toBe(4);
        subscription.unsubscribe();
    });

    it('keeps backing off when failures continue immediately after reconnecting', () => {
        const {source, counter} = alwaysFailingSource();
        const subscription = source.pipe(retryWithBackoff({initialDelayMs: 500, maxDelayMs: 1000, resetAfterMs: 900})).subscribe({error: () => undefined});
        jest.advanceTimersByTime(500);
        expect(counter.subscriptions).toBe(2);
        jest.advanceTimersByTime(1000);
        expect(counter.subscriptions).toBe(3);
        jest.advanceTimersByTime(999);
        expect(counter.subscriptions).toBe(3);
        jest.advanceTimersByTime(1);
        expect(counter.subscriptions).toBe(4);
        subscription.unsubscribe();
    });

    it('stops retrying once unsubscribed', () => {
        const {source, counter} = alwaysFailingSource();
        const subscription = source.pipe(retryWithBackoff({initialDelayMs: 500, maxDelayMs: 30000})).subscribe({error: () => undefined});
        subscription.unsubscribe();
        jest.advanceTimersByTime(60000);
        expect(counter.subscriptions).toBe(1);
    });
});
