import {GitUrl} from 'git-url-parse';

const GitUrlParse = require('git-url-parse');

function supportedSource(parsed: GitUrl): boolean {
    return parsed.resource.startsWith('github') || ['gitlab.com', 'bitbucket.org'].indexOf(parsed.source) >= 0;
}

function protocol(proto: string): string {
    return proto === 'ssh' ? 'https' : proto;
}

export function repoUrl(url: string): string {
    const parsed = GitUrlParse(url);

    if (!supportedSource(parsed)) {
        return null;
    }

    return `${protocol(parsed.protocol)}://${parsed.resource}/${parsed.owner}/${parsed.name}`;
}

export function revisionUrl(url: string, revision: string): string {

    const parsed = GitUrlParse(url);

    if (!supportedSource(parsed)) {
        return null;
    }

    return `${protocol(parsed.protocol)}://${parsed.resource}/${parsed.owner}/${parsed.name}/${(url.indexOf('bitbucket') >= 0 ? 'commits' : 'commit')}/${revision}`;
}
