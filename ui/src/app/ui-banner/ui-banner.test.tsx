import {AppContextReact} from 'argo-ui';
import * as React from 'react';
import {BehaviorSubject} from 'rxjs';
import {render, screen} from '@testing-library/react';
import '@testing-library/jest-dom';

import {services} from '../shared/services';
import {Banner} from './ui-banner';

// ui-banner only needs DataLoader from the shared barrel; mocking it avoids pulling in every shared component.
jest.mock('../shared/components', () => ({
    DataLoader: jest.requireActual('argo-ui').DataLoader,
    ErrorNotification: jest.requireActual('argo-ui').ErrorNotification
}));

jest.mock('../shared/services', () => ({
    services: {
        authService: {settings: jest.fn()},
        viewPreferences: {getPreferences: jest.fn(), updatePreferences: jest.fn()}
    }
}));

const showNotification = jest.fn();

const renderBanner = () =>
    render(
        <AppContextReact.Provider
            value={
                {
                    apis: {notifications: {show: showNotification}}
                } as any
            }>
            <Banner>
                <div>page content</div>
            </Banner>
        </AppContextReact.Provider>
    );

describe('Banner', () => {
    beforeEach(() => {
        jest.clearAllMocks();
        jest.mocked(services.viewPreferences.getPreferences).mockReturnValue(new BehaviorSubject({hideSidebar: false, hideBannerContent: null} as any));
    });

    // Regression: a /api/v1/settings response without `help` (an API server that predates the help links, or a
    // response proxied from something that isn't argocd-server) used to throw
    // "Cannot read properties of undefined (reading 'chatText')" and replace every page's content with
    // "Failed to load data, please try again."
    it('renders the page when settings has no help section', async () => {
        jest.mocked(services.authService.settings).mockResolvedValue({} as any);

        renderBanner();

        expect(await screen.findByText('page content')).toBeInTheDocument();
        expect(showNotification).not.toHaveBeenCalled();
    });

    it('renders the help chat link from settings', async () => {
        jest.mocked(services.authService.settings).mockResolvedValue({
            help: {chatText: 'Chat now!', chatUrl: 'https://example.com/chat', binaryUrls: {}}
        } as any);

        renderBanner();

        expect(await screen.findByRole('link', {name: /Chat now!/})).toHaveAttribute('href', 'https://example.com/chat');
    });
});
