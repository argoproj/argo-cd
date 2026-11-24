import * as React from 'react';
import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {act, fireEvent, render, screen, waitFor} from '@testing-library/react';
import '@testing-library/jest-dom';

import {notificationQueries} from '../../../shared/queries';
import {services} from '../../../shared/services';
import {EditNotificationSubscriptions, EditNotificationSubscriptionsProps} from './edit-notification-subscriptions';

jest.mock('../../../shared/services', () => ({services: {notification: {listTriggers: jest.fn(), listServices: jest.fn()}}}));
jest.mock('argo-ui', () => ({
    ...jest.requireActual('argo-ui'),
    Autocomplete: ({inputProps, value, onChange, items}: any) => <input {...inputProps} value={value} onChange={onChange} data-items={items.join(',')} />
}));

const subscription = {trigger: 'on-sync-succeeded', service: 'slack', value: 'my-channel'};
const renderEditor = (subscriptions = [subscription], client = new QueryClient({defaultOptions: {queries: {retry: false}}})) => {
    const onEditSubscription = jest.fn();
    render(
        <QueryClientProvider client={client}>
            <EditNotificationSubscriptions
                {...({subscriptions, onEditSubscription, onAddNewSubscription: jest.fn(), onRemoveSubscription: jest.fn()} as EditNotificationSubscriptionsProps)}
            />
        </QueryClientProvider>
    );
    return onEditSubscription;
};

describe('EditNotificationSubscriptions', () => {
    beforeEach(() => {
        jest.resetAllMocks();
        jest.mocked(services.notification.listTriggers).mockResolvedValue([{name: 'on-sync-succeeded'}] as any);
        jest.mocked(services.notification.listServices).mockResolvedValue([{name: 'slack'}] as any);
    });

    it('does not fetch suggestions until a subscription exists', () => {
        renderEditor([]);
        expect(screen.getByText('No items')).toBeInTheDocument();
        expect(services.notification.listTriggers).not.toHaveBeenCalled();
        expect(services.notification.listServices).not.toHaveBeenCalled();
    });

    it('loads suggestions once for multiple rows and preserves subscription values when editing', async () => {
        jest.mocked(services.notification.listTriggers).mockResolvedValue([{name: 'on-sync-running'}] as any);
        jest.mocked(services.notification.listServices).mockResolvedValue([{name: 'email'}] as any);
        const onEdit = renderEditor([subscription, {...subscription, value: 'another-channel'}]);
        const triggers = await screen.findAllByTitle('Trigger');
        const serviceInputs = await screen.findAllByTitle('Service');
        expect(triggers).toHaveLength(2);
        expect(triggers[0]).toHaveAttribute('data-items', 'on-sync-running');
        expect(serviceInputs[0]).toHaveAttribute('data-items', 'email');
        expect(services.notification.listTriggers).toHaveBeenCalledTimes(1);
        expect(services.notification.listServices).toHaveBeenCalledTimes(1);

        fireEvent.change(triggers[0], {target: {value: 'on-sync-failed'}});
        expect(onEdit).toHaveBeenCalledWith(0, {...subscription, trigger: 'on-sync-failed'});
    });

    it.each(['triggers', 'services'] as const)('retries failed %s without reloading the other suggestions', async resource => {
        const failed = resource === 'triggers' ? services.notification.listTriggers : services.notification.listServices;
        const other = resource === 'triggers' ? services.notification.listServices : services.notification.listTriggers;
        jest.mocked(failed).mockRejectedValueOnce(new Error('boom'));
        renderEditor();

        fireEvent.click(await screen.findByText('try again'));
        expect(await screen.findByTitle(resource === 'triggers' ? 'Trigger' : 'Service')).toBeInTheDocument();
        expect(failed).toHaveBeenCalledTimes(2);
        expect(other).toHaveBeenCalledTimes(1);
    });

    it('keeps cached suggestions and subscription values after a failed refresh', async () => {
        const client = new QueryClient({defaultOptions: {queries: {retry: false}}});
        renderEditor([subscription, {...subscription, value: 'another-channel'}], client);
        expect((await screen.findAllByTitle('Trigger'))[0]).toHaveValue(subscription.trigger);
        jest.mocked(services.notification.listTriggers)
            .mockRejectedValueOnce(new Error('boom'))
            .mockResolvedValueOnce([{name: 'on-sync-failed'}] as any);

        await act(() => client.invalidateQueries({queryKey: notificationQueries.triggers().queryKey}));
        const retry = await screen.findByText('try again');
        expect(screen.getAllByText('try again')).toHaveLength(1);
        expect(screen.getAllByTitle('Trigger')[0]).toHaveAttribute('data-items', 'on-sync-succeeded');
        expect(screen.getAllByTitle('Trigger')[0]).toHaveValue(subscription.trigger);
        expect(screen.getAllByTitle('Value')[0]).toHaveValue(subscription.value);
        fireEvent.click(retry);
        await waitFor(() => expect(screen.getAllByTitle('Trigger')[0]).toHaveAttribute('data-items', 'on-sync-failed'));
        expect(services.notification.listServices).toHaveBeenCalledTimes(1);
    });

    it('shows one refresh error and retries both failed lists', async () => {
        const client = new QueryClient({defaultOptions: {queries: {retry: false}}});
        renderEditor([subscription, {...subscription, value: 'another-channel'}], client);
        await screen.findAllByTitle('Trigger');
        await screen.findAllByTitle('Service');
        jest.mocked(services.notification.listTriggers).mockRejectedValueOnce(new Error('boom'));
        jest.mocked(services.notification.listServices).mockRejectedValueOnce(new Error('boom'));

        await act(() => client.invalidateQueries({queryKey: ['notifications']}));
        expect(await screen.findAllByText('try again')).toHaveLength(1);
        expect(screen.getAllByTitle('Trigger')).toHaveLength(2);
        expect(screen.getAllByTitle('Service')).toHaveLength(2);
        fireEvent.click(screen.getByText('try again'));
        await waitFor(() => expect(screen.queryByText('try again')).not.toBeInTheDocument());
        expect(services.notification.listTriggers).toHaveBeenCalledTimes(3);
        expect(services.notification.listServices).toHaveBeenCalledTimes(3);
    });
});
