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
        const onEdit = renderEditor([subscription, {...subscription, value: 'another-channel'}]);
        const triggers = await screen.findAllByTitle('Trigger');
        const serviceInputs = await screen.findAllByTitle('Service');
        expect(triggers).toHaveLength(2);
        expect(triggers[0]).toHaveAttribute('data-items', 'on-sync-succeeded');
        expect(serviceInputs[0]).toHaveAttribute('data-items', 'slack');
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
        renderEditor([subscription], client);
        expect(await screen.findByTitle('Trigger')).toHaveValue(subscription.trigger);
        jest.mocked(services.notification.listTriggers)
            .mockRejectedValueOnce(new Error('boom'))
            .mockResolvedValueOnce([{name: 'on-sync-failed'}] as any);

        await act(() => client.invalidateQueries({queryKey: notificationQueries.triggers().queryKey}));
        const retry = await screen.findByText('try again');
        expect(screen.getByTitle('Trigger')).toHaveAttribute('data-items', 'on-sync-succeeded');
        expect(screen.getByTitle('Trigger')).toHaveValue(subscription.trigger);
        expect(screen.getByTitle('Value')).toHaveValue(subscription.value);
        fireEvent.click(retry);
        await waitFor(() => expect(screen.getByTitle('Trigger')).toHaveAttribute('data-items', 'on-sync-failed'));
    });
});
