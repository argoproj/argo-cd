import {GitUrl} from 'git-url-parse';
import {isSHA} from './revision';
import {isValidURL} from '../../shared/utils';

const GitUrlParse = require('git-url-parse');

// Returns true for self-hosted Bitbucket Server instances.
// git-url-parse sets source='bitbucket-server' for HTTPS clone URLs (which contain /scm/
// in their path) and for SCP-style SSH clone URLs that also include /scm/. For SSH clone
// URLs without the /scm/ prefix (e.g. ssh://git@HOST:7999/PROJECT/repo.git) it does not
// set that source, so we additionally check the resource hostname.
function isBitbucketServer(parsed: GitUrl): boolean {
    return parsed.source === 'bitbucket-server' ||
        (parsed.resource.startsWith('bitbucket') && parsed.source !== 'bitbucket.org');
}

function supportedSource(parsed: GitUrl): boolean {
    return parsed.resource.startsWith('github') || parsed.source === 'bitbucket.org' ||
        isBitbucketServer(parsed) || parsed.source === 'gitlab.com';
}

// Bitbucket Server browse URLs differ from clone URLs:
//   clone:  https://HOST/scm/~user/repo.git  or  https://HOST/scm/PROJECTKEY/repo.git
//   browse: https://HOST/users/user/repos/repo  or  https://HOST/projects/PROJECTKEY/repos/repo
function bitbucketServerBrowseUrl(parsed: GitUrl): string {
    const host = `https://${parsed.resource}`;
    const repoName = parsed.name;
    const owner = parsed.owner;
    // Personal repos use the ~ prefix in the clone URL owner
    if (owner.startsWith('~')) {
        return `${host}/users/${owner.slice(1)}/repos/${repoName}`;
    }
    return `${host}/projects/${owner}/repos/${repoName}`;
}

function protocol(proto: string): string {
    return proto === 'ssh' ? 'https' : proto;
}

export function repoUrl(url: string): string {
    try {
        const parsed = GitUrlParse(url);

        if (!supportedSource(parsed)) {
            return null;
        }

        // Self-hosted Bitbucket Server has a different browse URL structure
        // from its clone URLs, so reconstruct it using the known pattern.
        if (isBitbucketServer(parsed)) {
            const browseUrl = bitbucketServerBrowseUrl(parsed);
            if (isValidURL(browseUrl)) {
                return browseUrl;
            }
            return null;
        }

        const parsedUrl = `${protocol(parsed.protocol)}://${parsed.resource}/${parsed.owner}/${parsed.name}`;
        if (!isValidURL(parsedUrl)) {
            return null;
        }
        return parsedUrl;
    } catch {
        return null;
    }
}

export function revisionUrl(url: string, revision: string, forPath: boolean): string {
    let parsed;
    try {
        parsed = GitUrlParse(url);
    } catch {
        return null;
    }

    if (!supportedSource(parsed)) {
        return null;
    }

    // Bitbucket Server uses /commits/SHA for bare commit links, and /browse[/PATH]?at=REF
    // for branch and path links. The Revision component (revision.tsx) inserts the path
    // segment BEFORE the ?at= query param, so we return only the base browse URL here.
    // When revision is empty or HEAD, omit ?at= so Bitbucket Server uses its default branch.
    if (isBitbucketServer(parsed)) {
        const base = bitbucketServerBrowseUrl(parsed);
        if (isSHA(revision) && !forPath) {
            return `${base}/commits/${revision}`;
        }
        const ref = revision || '';
        if (!ref || ref === 'HEAD') {
            return `${base}/browse`;
        }
        const atRef = isSHA(revision) ? revision : `refs/heads/${revision}`;
        return `${base}/browse?at=${encodeURIComponent(atRef)}`;
    }

    let urlSubPath = isSHA(revision) ? 'commit' : 'tree';

    if (parsed.source === 'bitbucket.org') {
        // The reason for the condition of 'forPath' is that when we build nested path, we need to use 'src'
        urlSubPath = isSHA(revision) && !forPath ? 'commits' : 'src';
    }

    // Gitlab changed the way urls to commit look like
    // Ref: https://docs.gitlab.com/ee/update/deprecations.html#legacy-urls-replaced-or-removed
    if (parsed.source === 'gitlab.com') {
        urlSubPath = '-/' + urlSubPath;
    }

    return `${protocol(parsed.protocol)}://${parsed.resource}/${parsed.owner}/${parsed.name}/${urlSubPath}/${revision || 'HEAD'}`;
}
