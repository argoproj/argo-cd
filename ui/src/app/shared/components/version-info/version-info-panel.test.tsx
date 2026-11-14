import {QueryClientProvider} from '@tanstack/react-query';
import {act, cleanup, fireEvent, render, screen, waitFor} from '@testing-library/react';
import {createMemoryHistory} from 'history';
import * as React from 'react';

import {Sidebar} from '../../../sidebar/sidebar';
import {Provider} from '../../context';
import {VersionMessage} from '../../models';
import {versionQueries} from '../../queries';
import {newQueryClient} from '../../query-client';
import {services} from '../../services';
import {VersionPanel} from './version-info-panel';

jest.mock('argo-ui', () => ({...jest.requireActual('argo-ui'), Tooltip: ({children}: React.PropsWithChildren) => children}));
jest.mock('../../services', () => {
    const {ViewPreferencesService} = jest.requireActual('../../services/view-preferences-service');
    return {services: {version: {version: jest.fn()}, viewPreferences: new ViewPreferencesService()}};
});

const version: VersionMessage = {
    Version: 'v3.7.0',
    BuildDate: '2026-11-12T08:16:10Z',
    GoVersion: 'go1.27.0',
    Compiler: 'gc',
    Platform: 'linux/arm64',
    KustomizeVersion: 'v5.7.1',
    HelmVersion: 'v3.19.0',
    KubectlVersion: 'v1.34.0',
    JsonnetVersion: 'v0.21.0'
};

describe('VersionPanel', () => {
    let client: ReturnType<typeof newQueryClient>;
    let notifications: {show: jest.Mock};
    const onClose = jest.fn();

    beforeAll(() => services.viewPreferences.init());

    beforeEach(() => {
        jest.resetAllMocks();
        services.viewPreferences.updatePreferences({theme: 'light'});
        notifications = {show: jest.fn()};
        client = newQueryClient(notifications as any);
    });

    afterEach(() => {
        cleanup();
        client.clear();
        jest.restoreAllMocks();
    });

    function renderPanel(isShown = true, sidebar = false) {
        const content = (shown: boolean) => (
            <QueryClientProvider client={client}>
                <Provider value={{history: createMemoryHistory(), baseHref: '/'} as any}>
                    {sidebar && <Sidebar navItems={[]} pref={services.viewPreferences.getCurrentPreferences()} onVersionClick={jest.fn()} />}
                    <VersionPanel isShown={shown} onClose={onClose} />
                </Provider>
            </QueryClientProvider>
        );
        const view = render(content(isShown));
        return {...view, setShown: (shown: boolean) => view.rerender(content(shown))};
    }

    it('waits until opened, shares the sidebar request, and renders every version field', async () => {
        let resolveVersion: (value: VersionMessage) => void;
        jest.mocked(services.version.version).mockImplementation(() => new Promise(resolve => (resolveVersion = resolve)));
        const view = renderPanel(false);
        expect(services.version.version).not.toHaveBeenCalled();
        view.unmount();

        const {setShown} = renderPanel(false, true);
        expect(services.version.version).toHaveBeenCalledTimes(1);
        setShown(true);
        expect(screen.getAllByText('Loading...')).toHaveLength(2);
        await act(async () => resolveVersion(version));

        await waitFor(() => expect(screen.getAllByText(version.Version)).toHaveLength(2));
        for (const value of Object.values(version).slice(1)) {
            expect(screen.getByText(value)).toBeInTheDocument();
        }
        expect(services.version.version).toHaveBeenCalledTimes(1);
        fireEvent.click(screen.getByRole('button', {name: 'Close'}));
        expect(onClose).toHaveBeenCalledTimes(1);
    });

    it('offers retry after an initial failure and shows loading while retrying', async () => {
        let resolveRetry: (value: VersionMessage) => void;
        jest.mocked(services.version.version)
            .mockRejectedValueOnce(new Error('unavailable'))
            .mockImplementationOnce(() => new Promise(resolve => (resolveRetry = resolve)));
        renderPanel();
        fireEvent.click(await screen.findByRole('button', {name: 'try again'}));
        expect(await screen.findByText('Loading...')).toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Copy JSON'})).not.toBeInTheDocument();

        await act(async () => resolveRetry(version));
        expect(await screen.findByText(version.Version)).toBeInTheDocument();
        expect(services.version.version).toHaveBeenCalledTimes(2);
        expect(notifications.show).not.toHaveBeenCalled();
    });

    it('keeps cached details after a failed refresh and updates them on retry', async () => {
        client.setQueryData(versionQueries.get().queryKey, version);
        jest.mocked(services.version.version)
            .mockRejectedValueOnce(new Error('unavailable'))
            .mockResolvedValueOnce({...version, Version: 'v3.7.1'});
        renderPanel();
        expect(screen.getByText(version.Version)).toBeInTheDocument();
        fireEvent.click(await screen.findByRole('button', {name: 'try again'}));

        expect(screen.getByText(version.Version)).toBeInTheDocument();
        expect(await screen.findByText('v3.7.1')).toBeInTheDocument();
        expect(screen.queryByText('try again')).not.toBeInTheDocument();
        expect(notifications.show).not.toHaveBeenCalled();
    });

    it('reacts to theme changes without fetching the version again', async () => {
        jest.mocked(services.version.version).mockResolvedValue(version);
        const {container} = renderPanel();
        await screen.findByText(version.Version);
        expect(container.querySelector('.theme-light')).toBeInTheDocument();

        act(() => services.viewPreferences.updatePreferences({theme: 'dark'}));

        expect(container.querySelector('.theme-dark')).toBeInTheDocument();
        expect(services.version.version).toHaveBeenCalledTimes(1);
    });

    it.each([false, true])('copies the displayed JSON and reports clipboard failure: %s', async failed => {
        const writeText = jest.fn().mockImplementation(() => (failed ? Promise.reject(new Error('denied')) : Promise.resolve()));
        Object.defineProperty(navigator, 'clipboard', {configurable: true, value: {writeText}});
        jest.mocked(services.version.version).mockResolvedValue(version);
        renderPanel();

        fireEvent.click(await screen.findByRole('button', {name: 'Copy JSON'}));

        expect(writeText).toHaveBeenCalledWith(JSON.stringify(version, undefined, 4) + '\n');
        expect(await screen.findByRole('button', {name: failed ? 'Copy Failed' : 'Copied'})).toBeInTheDocument();
        await waitFor(() => expect(screen.getByRole('button', {name: 'Copy JSON'})).toBeInTheDocument());
    });
});
