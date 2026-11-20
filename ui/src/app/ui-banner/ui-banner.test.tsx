import * as React from 'react';
import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {act, fireEvent, render, screen, waitFor} from '@testing-library/react';
import '@testing-library/jest-dom';

import {services} from '../shared/services';
import {Banner} from './ui-banner';

jest.mock('argo-ui', () => ({...jest.requireActual('argo-ui'), Tooltip: (props: {children: React.ReactNode}) => props.children}));
jest.mock('../shared/services', () => {
    const {ViewPreferencesService} = jest.requireActual('../shared/services/view-preferences-service');
    return {services: {authService: {settings: jest.fn()}, viewPreferences: new ViewPreferencesService()}};
});

const content = 'Maintenance on Saturday';

const mockSettings = (settings: object) => jest.mocked(services.authService.settings).mockResolvedValue({help: {}, ...settings} as any);

const renderBanner = () =>
    render(
        <QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>
            <Banner>
                <div>app content</div>
            </Banner>
        </QueryClientProvider>
    );

const bannerVisibility = () => (screen.getByText(content).closest('.ui-banner') as HTMLElement).style.visibility;

describe('Banner', () => {
    beforeEach(() => {
        jest.clearAllMocks();
        window.localStorage.clear();
        services.viewPreferences.init();
        services.viewPreferences.updatePreferences({hideBannerContent: null});
    });

    it('shows the banner and its link once settings load', async () => {
        mockSettings({uiBannerContent: content, uiBannerURL: 'https://status.example.com'});
        renderBanner();

        expect(await screen.findByRole('link', {name: content})).toHaveAttribute('href', 'https://status.example.com');
        expect(bannerVisibility()).toBe('visible');
        expect(screen.getByText('app content')).toBeInTheDocument();
    });

    it('shows a loading message until settings load', async () => {
        let resolveSettings: (settings: any) => void;
        jest.mocked(services.authService.settings).mockReturnValueOnce(new Promise(resolve => (resolveSettings = resolve)));
        renderBanner();

        expect(screen.getByText('Loading...')).toBeInTheDocument();
        expect(screen.queryByText('app content')).not.toBeInTheDocument();

        await act(async () => resolveSettings({help: {}, uiBannerContent: content}));

        expect(await screen.findByText('app content')).toBeInTheDocument();
        expect(screen.queryByText('Loading...')).not.toBeInTheDocument();
    });

    it('hides the banner and remembers it after "Don\'t show again"', async () => {
        mockSettings({uiBannerContent: content});
        renderBanner();

        fireEvent.click(await screen.findByText("Don't show again"));

        expect(bannerVisibility()).toBe('hidden');
        expect(services.viewPreferences.getCurrentPreferences().hideBannerContent).toBe(content);
    });

    it('hides the banner for now after "Dismiss for now", without remembering it', async () => {
        mockSettings({uiBannerContent: content});
        renderBanner();

        fireEvent.click(await screen.findByText('Dismiss for now'));

        expect(bannerVisibility()).toBe('hidden');
        expect(services.viewPreferences.getCurrentPreferences().hideBannerContent).toBeNull();
    });

    it('always shows a permanent banner, without dismiss buttons', async () => {
        services.viewPreferences.updatePreferences({hideBannerContent: content});
        mockSettings({uiBannerContent: content, uiBannerPermanent: true});
        renderBanner();

        await screen.findByText(content);
        expect(bannerVisibility()).toBe('visible');
        expect(screen.queryByText("Don't show again")).not.toBeInTheDocument();
    });

    it('forgets a hidden banner once the server no longer sets one', async () => {
        services.viewPreferences.updatePreferences({hideBannerContent: content});
        mockSettings({});
        renderBanner();

        expect(await screen.findByText('app content')).toBeInTheDocument();
        await waitFor(() => expect(services.viewPreferences.getCurrentPreferences().hideBannerContent).toBeNull());
    });

    it('shows the chat button with the configured text', async () => {
        mockSettings({help: {chatUrl: 'https://chat.example.com', chatText: 'Chat now'}});
        renderBanner();

        expect(await screen.findByRole('link', {name: /Chat now/})).toHaveAttribute('href', 'https://chat.example.com');
    });

    it('hides the banner when it is hidden elsewhere, such as in another tab', async () => {
        mockSettings({uiBannerContent: content});
        renderBanner();
        await screen.findByText(content);
        expect(bannerVisibility()).toBe('visible');

        act(() => services.viewPreferences.updatePreferences({hideBannerContent: content}));

        expect(bannerVisibility()).toBe('hidden');
    });

    it('offers to try again when settings fail to load, then shows the app', async () => {
        jest.mocked(services.authService.settings).mockRejectedValueOnce(new Error('boom'));
        let resolveSettings: (settings: any) => void;
        jest.mocked(services.authService.settings).mockReturnValueOnce(new Promise(resolve => (resolveSettings = resolve)));
        renderBanner();

        fireEvent.click(await screen.findByText('try again'));

        expect(await screen.findByText('Loading...')).toBeInTheDocument();
        expect(screen.queryByText('app content')).not.toBeInTheDocument();

        await act(async () => resolveSettings({help: {}, uiBannerContent: content}));

        expect(await screen.findByText('app content')).toBeInTheDocument();
        expect(screen.queryByText('Loading...')).not.toBeInTheDocument();
        expect(screen.getByText(content)).toBeInTheDocument();
        expect(services.authService.settings).toHaveBeenCalledTimes(2);
    });
});
