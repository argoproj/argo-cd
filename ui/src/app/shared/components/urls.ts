const GitUrlParse = require('git-url-parse');

function supportedSource(source: string): boolean {
    return ['github.com', 'gitlab.com', 'bitbucket.org'].indexOf(source) >= 0;
}

function protocol(proto: string): string {
    return proto === 'ssh' ? 'https' : proto;
}

export function repoUrl(url: string): string {
    const parsed = GitUrlParse(url);

    if (!supportedSource(parsed.source)) {
        return null;
    }

    return `${protocol(parsed.protocol)}://${parsed.resource}/${parsed.owner}/${parsed.name}`;
}

export function revisionUrl(url: string, revision: string): string {

    const parsed = GitUrlParse(url);

    if (!supportedSource(parsed.source)) {
        return null;
    }

    return `${protocol(parsed.protocol)}://${parsed.resource}/${parsed.owner}/${parsed.name}/${(url.indexOf('bitbucket') >= 0 ? 'commits' : 'commit')}/${revision}`;
}
