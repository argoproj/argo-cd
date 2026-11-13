import {newQueryClient} from './query-client';

describe('newQueryClient', () => {
    let notifications: {show: jest.Mock};
    let client: ReturnType<typeof newQueryClient>;

    beforeEach(() => {
        notifications = {show: jest.fn()};
        client = newQueryClient(notifications as any);
    });

    afterEach(() => client.clear());

    const fail = (e: any, meta?: {silent?: boolean}) => client.fetchQuery({queryKey: ['k'], queryFn: () => Promise.reject(e), meta}).catch((): void => undefined);

    it('notifies on a failed load', async () => {
        await fail({status: 500});
        expect(notifications.show).toHaveBeenCalledTimes(1);
    });

    it('does not notify when a query opts out', async () => {
        await fail({status: 500}, {silent: true});
        expect(notifications.show).not.toHaveBeenCalled();
    });

    it('still notifies when silent is false', async () => {
        await fail({status: 500}, {silent: false});
        expect(notifications.show).toHaveBeenCalledTimes(1);
    });

    it('does not notify on 401', async () => {
        await fail({status: 401});
        expect(notifications.show).not.toHaveBeenCalled();
    });

    it('does not retry', async () => {
        const queryFn = jest.fn(() => Promise.reject(new Error('boom')));
        await client.fetchQuery({queryKey: ['k'], queryFn}).catch((): void => undefined);
        expect(queryFn).toHaveBeenCalledTimes(1);
    });
});
