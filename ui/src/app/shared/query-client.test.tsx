import {newQueryClient} from './query-client';

describe('newQueryClient', () => {
    const fail = (client: ReturnType<typeof newQueryClient>, e: any) => client.fetchQuery({queryKey: ['k'], queryFn: () => Promise.reject(e)}).catch((): void => undefined);

    it('notifies on a failed load', async () => {
        const notifications = {show: jest.fn()} as any;
        await fail(newQueryClient(notifications), {status: 500});
        expect(notifications.show).toHaveBeenCalledTimes(1);
    });

    it('does not notify on 401', async () => {
        const notifications = {show: jest.fn()} as any;
        await fail(newQueryClient(notifications), {status: 401});
        expect(notifications.show).not.toHaveBeenCalled();
    });

    it('does not retry', async () => {
        const queryFn = jest.fn(() => Promise.reject(new Error('boom')));
        await newQueryClient({show: jest.fn()} as any)
            .fetchQuery({queryKey: ['k'], queryFn})
            .catch((): void => undefined);
        expect(queryFn).toHaveBeenCalledTimes(1);
    });
});
