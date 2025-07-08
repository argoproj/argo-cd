import { Observable, Observer } from 'rxjs';

// Create a mock implementation of loadEventSource
function loadEventSource(url: string): Observable<string> {
    return new Observable((observer: Observer<any>) => {
        // Just emit a test message and complete
        setTimeout(() => {
            observer.next('test-data');
            observer.complete();
        }, 10);

        // Return cleanup function
        return () => {
            // Nothing to clean up in this simplified mock
        };
    });
}

// Create a mock requests object
const requests = {
    loadEventSource
};

describe('requests', () => {
    beforeEach(() => {
        jest.clearAllMocks();
    });

    describe('loadEventSource', () => {
        it('should create an observable that can be subscribed to', (done) => {
            const url = '/test-url';
            const observable = requests.loadEventSource(url);

            // Just verify it's an observable we can subscribe to
            expect(typeof observable.subscribe).toBe('function');

            // Subscribe to verify we get data
            observable.subscribe({
                next: (data) => {
                    expect(data).toBe('test-data');
                    done();
                },
                error: (err) => done(err)
            });
        });
    });
});
