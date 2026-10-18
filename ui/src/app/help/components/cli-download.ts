// The Help page used to offer a single "Linux" download link, whichever platform the browser ran on.
// This module decides which CLI downloads to advertise, so the choice can be unit tested.

export type ClientOS = 'linux' | 'macos' | 'windows' | 'unknown';

export interface CliDownload {
    /** Stable key for React lists. */
    key: string;
    label: string;
    href: string;
    /** Font Awesome icon class, without the leading "fa". */
    icon: string;
}

export interface CliDownloadOptions {
    os: ClientOS;
    /** Architecture of the argocd-server that serves the download, e.g. "amd64". */
    hostArch?: string;
    /** Version reported by argocd-server, e.g. "v3.5.0" or "v3.5.0+0dc5b3f". */
    version?: string;
    /** Size in bytes of the binary served by argocd-server, when known. */
    serverBinarySize?: number;
    /** Binary URLs configured in argocd-cm under `help.download.<os>-<arch>`. */
    binaryUrls?: Record<string, string>;
}

const RELEASES_URL = 'https://github.com/argoproj/argo-cd/releases';

/** Path of the binary argocd-server serves from its own image. */
export const SERVER_BINARY_URL = 'download/argocd-linux';

/** Detects the operating system the UI runs on, so the matching download is offered first. */
export function clientOS(userAgent: string, platform: string): ClientOS {
    const ua = (userAgent || '').toLowerCase();
    const plat = (platform || '').toLowerCase();
    if (ua.includes('windows') || plat.startsWith('win')) {
        return 'windows';
    }
    if (ua.includes('mac os') || ua.includes('macintosh') || plat.startsWith('mac')) {
        return 'macos';
    }
    if (ua.includes('linux') || ua.includes('x11') || plat.startsWith('linux')) {
        return 'linux';
    }
    return 'unknown';
}

/**
 * Release page for the running version. Build metadata ("+0dc5b3f") means the server runs an
 * untagged build, so there is no tag page to link to.
 */
export function releasesUrl(version: string): string {
    const release = /^v\d+\.\d+\.\d+$/.exec((version || '').trim());
    return release ? `${RELEASES_URL}/tag/${release[0]}` : `${RELEASES_URL}/latest`;
}

/** Human readable size for the download hint, e.g. "209 MB". */
export function formatBytes(bytes: number): string {
    if (!Number.isFinite(bytes) || bytes <= 0) {
        return '';
    }
    const mb = bytes / 1_000_000;
    return mb >= 1 ? `${Math.round(mb)} MB` : `${Math.max(1, Math.round(bytes / 1000))} kB`;
}

/**
 * Downloads to show on the Help page, most relevant first.
 *
 * The image only ships the Linux binary of its own architecture, so that link cannot be tailored
 * to macOS or Windows clients; those users get the GitHub release page instead, which offers every
 * OS/architecture. When the browser runs on Linux the direct download comes first, since it is the
 * only way to get the exact version the server runs.
 */
export function cliDownloads(opts: CliDownloadOptions): CliDownload[] {
    const hint = [opts.hostArch, formatBytes(opts.serverBinarySize || 0)].filter(Boolean).join(', ');
    const serverBinary: CliDownload = {
        key: 'server',
        label: hint ? `Linux (${hint})` : 'Linux',
        href: SERVER_BINARY_URL,
        icon: 'fa-linux'
    };
    const releases: CliDownload = {
        key: 'releases',
        label: 'All platforms',
        href: releasesUrl(opts.version || ''),
        icon: 'fa-github'
    };

    const configured: CliDownload[] = Object.keys(opts.binaryUrls || {})
        .sort()
        .map(binaryName => {
            const match = binaryName.match(/(darwin|windows|linux)-(amd64|arm64|ppc64le|s390x)/);
            const platform = match ? match[1] : '';
            const archName = match ? match[2] : '';
            return {
                key: `configured-${binaryName}`,
                label: `${platform || binaryName}${archName ? ` (${archName})` : ''}`,
                href: opts.binaryUrls[binaryName],
                icon: platform === 'windows' ? 'fa-windows' : platform === 'darwin' ? 'fa-apple' : 'fa-linux'
            };
        });

    const primary = opts.os === 'linux' ? [serverBinary, releases] : [releases, serverBinary];
    return [...primary, ...configured];
}
