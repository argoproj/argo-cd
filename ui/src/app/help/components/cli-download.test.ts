import {cliDownloads, clientOS, formatBytes, releasesUrl} from './cli-download';

describe('clientOS', () => {
    test.each([
        ['Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36', 'MacIntel', 'macos'],
        ['Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36', 'Win32', 'windows'],
        ['Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36', 'Linux x86_64', 'linux'],
        ['', '', 'unknown']
    ])('detects %s', (userAgent: string, platform: string, expected: string) => {
        expect(clientOS(userAgent, platform)).toEqual(expected);
    });
});

describe('releasesUrl', () => {
    test('links to the release tag', () => {
        expect(releasesUrl('v3.4.1')).toEqual('https://github.com/argoproj/argo-cd/releases/tag/v3.4.1');
    });

    test('falls back to the latest release for untagged builds', () => {
        expect(releasesUrl('v3.5.0+0dc5b3f')).toEqual('https://github.com/argoproj/argo-cd/releases/latest');
        expect(releasesUrl('')).toEqual('https://github.com/argoproj/argo-cd/releases/latest');
    });
});

describe('formatBytes', () => {
    test('reports megabytes', () => {
        expect(formatBytes(209_000_000)).toEqual('209 MB');
    });

    test('reports kilobytes', () => {
        expect(formatBytes(900_000)).toEqual('900 kB');
    });

    test('is empty when the size is unknown', () => {
        expect(formatBytes(0)).toEqual('');
        expect(formatBytes(NaN)).toEqual('');
    });
});

describe('cliDownloads', () => {
    test('offers the releases page to non-Linux clients', () => {
        const downloads = cliDownloads({os: 'macos', hostArch: 'amd64', version: 'v3.4.1'});
        expect(downloads[0]).toEqual({
            key: 'releases',
            label: 'All platforms',
            href: 'https://github.com/argoproj/argo-cd/releases/tag/v3.4.1',
            icon: 'fa-github'
        });
        expect(downloads.map(download => download.key)).toEqual(['releases', 'server']);
    });

    test('offers the server binary first on Linux, with its arch and size', () => {
        const downloads = cliDownloads({os: 'linux', hostArch: 'amd64', version: 'v3.4.1', serverBinarySize: 209_000_000});
        expect(downloads[0]).toEqual({
            key: 'server',
            label: 'Linux (amd64, 209 MB)',
            href: 'download/argocd-linux',
            icon: 'fa-linux'
        });
        expect(downloads.map(download => download.key)).toEqual(['server', 'releases']);
    });

    test('keeps the binary URLs configured in argocd-cm', () => {
        const downloads = cliDownloads({
            os: 'windows',
            hostArch: 'amd64',
            version: 'v3.4.1',
            binaryUrls: {'linux-arm64': 'https://example.com/argocd-linux-arm64', 'darwin-arm64': 'https://example.com/argocd-darwin-arm64'}
        });
        expect(downloads.map(download => download.label)).toEqual(['All platforms', 'Linux (amd64)', 'darwin (arm64)', 'linux (arm64)']);
        expect(downloads[2]).toEqual({key: 'configured-darwin-arm64', label: 'darwin (arm64)', href: 'https://example.com/argocd-darwin-arm64', icon: 'fa-apple'});
    });
});
