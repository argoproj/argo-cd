import {cliDownloadURL, RELEASES_URL} from './help';

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
