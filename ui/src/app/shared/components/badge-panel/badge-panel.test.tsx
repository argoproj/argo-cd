import * as React from 'react';
import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {act, fireEvent, render, screen, waitFor, waitForElementToBeRemoved} from '@testing-library/react';
import '@testing-library/jest-dom';

import {Provider} from '../../context';
import {settingsQueries} from '../../queries';
import {services} from '../../services';
import {BadgePanel} from './badge-panel';

jest.mock('../../services', () => ({services: {authService: {settings: jest.fn()}}}));

const renderBadges = (children: React.ReactNode, client = new QueryClient({defaultOptions: {queries: {retry: false}}})) =>
    render(
        <QueryClientProvider client={client}>
            <Provider value={{baseHref: '/'} as any}>{children}</Provider>
        </QueryClientProvider>
    );

describe('BadgePanel', () => {
    beforeEach(() => jest.clearAllMocks());

    it('shares settings across application and project badges', async () => {
        jest.mocked(services.authService.settings).mockResolvedValue({statusBadgeEnabled: true, statusBadgeRootUrl: 'https://badges.example/'} as any);
        renderBadges(
            <>
                <BadgePanel app='my-app' appNamespace='my-ns' nsEnabled={true} />
                <BadgePanel project='my-project' />
            </>
        );

        expect(screen.getAllByText('Loading...')).toHaveLength(2);
        expect(await screen.findAllByText('STATUS BADGE')).toHaveLength(2);
        expect(screen.getAllByRole('textbox')[0]).toHaveValue('https://badges.example/api/badge?name=my-app&revision=true&showAppName=true&namespace=my-ns');
        expect(screen.getAllByRole('textbox')[1]).toHaveValue('https://badges.example/api/badge?project=my-project&revision=true');
        expect(services.authService.settings).toHaveBeenCalledTimes(1);
    });

    it('hides badges when disabled', async () => {
        jest.mocked(services.authService.settings).mockResolvedValue({statusBadgeEnabled: false} as any);
        renderBadges(<BadgePanel app='my-app' />);

        await waitForElementToBeRemoved(() => screen.queryByText('Loading...'));
        expect(screen.queryByText('STATUS BADGE')).not.toBeInTheDocument();
    });

    it('offers a retry when settings fail to load', async () => {
        jest.mocked(services.authService.settings)
            .mockRejectedValueOnce(new Error('boom'))
            .mockResolvedValueOnce({statusBadgeEnabled: true, statusBadgeRootUrl: ''} as any);
        renderBadges(<BadgePanel app='my-app' />);

        fireEvent.click(await screen.findByText('try again'));

        expect(await screen.findByText('STATUS BADGE')).toBeInTheDocument();
        expect(services.authService.settings).toHaveBeenCalledTimes(2);
    });

    it('keeps a cached badge visible and offers retry after refetch fails', async () => {
        jest.mocked(services.authService.settings)
            .mockResolvedValueOnce({statusBadgeEnabled: true, statusBadgeRootUrl: '/old/'} as any)
            .mockRejectedValueOnce(new Error('boom'))
            .mockResolvedValueOnce({statusBadgeEnabled: true, statusBadgeRootUrl: '/new/'} as any);
        const client = new QueryClient({defaultOptions: {queries: {retry: false}}});
        renderBadges(<BadgePanel app='my-app' />, client);
        expect(await screen.findByRole('textbox')).toHaveValue('/old/api/badge?name=my-app&revision=true&showAppName=true');

        await act(() => client.invalidateQueries({queryKey: settingsQueries.get().queryKey}));
        expect(await screen.findByText('try again')).toBeInTheDocument();
        expect(screen.getByRole('textbox')).toHaveValue('/old/api/badge?name=my-app&revision=true&showAppName=true');

        fireEvent.click(screen.getByText('try again'));
        await waitFor(() => expect(screen.getByRole('textbox')).toHaveValue('/new/api/badge?name=my-app&revision=true&showAppName=true'));
    });
});
