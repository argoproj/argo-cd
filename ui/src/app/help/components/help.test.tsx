import * as React from 'react';
import {render, screen} from '@testing-library/react';
import {Help, cliDownloadURL, RELEASES_URL} from './help';
import {services} from '../../shared/services';

jest.mock('../../shared/services', () => ({
    services: {
        authService: {settings: jest.fn()},
        version: {version: jest.fn()}
    }
}));

// Render Page as a passthrough so the test exercises the download-button logic without
// pulling in the full navigation/context stack; DataLoader stays real to drive the load.
jest.mock('../../shared/components', () => ({
    DataLoader: jest.requireActual('argo-ui').DataLoader,
    Page: ({children}: {children: React.ReactNode}) => <div>{children}</div>
}));

const mockSettings = (binaryUrls?: Record<string, string>) => (services.authService.settings as jest.Mock).mockResolvedValue({help: {binaryUrls}});
const mockVersion = (Version: string) => (services.version.version as jest.Mock).mockResolvedValue({Version});

test('links released versions to their GitHub release tag', () => {
    expect(cliDownloadURL('v3.4.1')).toBe(`${RELEASES_URL}/tag/v3.4.1`);
    expect(cliDownloadURL('v3.4.0-rc1')).toBe(`${RELEASES_URL}/tag/v3.4.0-rc1`);
});

test('falls back to the releases list for dev builds and unknown versions', () => {
    expect(cliDownloadURL('v3.5.0+0dc5b3f')).toBe(RELEASES_URL);
    expect(cliDownloadURL('')).toBe(RELEASES_URL);
    expect(cliDownloadURL(undefined as unknown as string)).toBe(RELEASES_URL);
    expect(cliDownloadURL('latest')).toBe(RELEASES_URL);
});

describe('Help CLI download buttons', () => {
    beforeEach(() => jest.clearAllMocks());

    test('shows the GitHub releases link for the running version when no download keys are configured', async () => {
        mockSettings(undefined);
        mockVersion('v3.4.1');
        render(<Help />);
        const link = await screen.findByText('GitHub releases');
        expect(link.closest('a')).toHaveAttribute('href', `${RELEASES_URL}/tag/v3.4.1`);
    });

    test('renders configured download buttons alongside the GitHub releases link', async () => {
        mockSettings({'darwin-arm64': 'https://example.com/argocd-darwin-arm64'});
        mockVersion('v3.4.1');
        render(<Help />);
        const link = await screen.findByText(/darwin/);
        expect(link.closest('a')).toHaveAttribute('href', 'https://example.com/argocd-darwin-arm64');
        expect(screen.getByText('GitHub releases').closest('a')).toHaveAttribute('href', `${RELEASES_URL}/tag/v3.4.1`);
    });
});
