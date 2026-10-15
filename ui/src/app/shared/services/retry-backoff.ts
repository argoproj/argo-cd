import {defer, MonoTypeOperatorFunction, timer} from 'rxjs';
import {retry} from 'rxjs/operators';

export interface RetryBackoffOptions {
    initialDelayMs?: number;
    maxDelayMs?: number;
    resetAfterMs?: number;
}

const DEFAULT_INITIAL_DELAY_MS = 500;
const DEFAULT_MAX_DELAY_MS = 30000;
const DEFAULT_RESET_AFTER_MS = 60000;

export function retryWithBackoff<T>(options: RetryBackoffOptions = {}): MonoTypeOperatorFunction<T> {
    const initialDelayMs = options.initialDelayMs ?? DEFAULT_INITIAL_DELAY_MS;
    const maxDelayMs = options.maxDelayMs ?? DEFAULT_MAX_DELAY_MS;
    const resetAfterMs = options.resetAfterMs ?? DEFAULT_RESET_AFTER_MS;
    const delayFor = (attempt: number) => Math.min(initialDelayMs * 2 ** (attempt - 1), maxDelayMs);
    return source =>
        defer(() => {
            let attempt = 0;
            let subscribedAt = 0;
            return defer(() => {
                subscribedAt = Date.now();
                return source;
            }).pipe(
                retry({
                    delay: () => {
                        attempt = attempt > 0 && Date.now() - subscribedAt >= resetAfterMs ? 1 : attempt + 1;
                        return timer(delayFor(attempt));
                    }
                })
            );
        });
}
