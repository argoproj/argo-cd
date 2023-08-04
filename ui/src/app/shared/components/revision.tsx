import * as React from 'react';
import {revisionUrl} from './urls';

export const Revision = ({repoUrl, revision, children}: {repoUrl: string; revision: string; children?: React.ReactNode}) => {
    revision = revision || '';
    const url = revisionUrl(repoUrl, revision);
    const content = children || (isSHA(revision) ? revision.substr(0, 7) : revision);
    return url !== null ? (
        <a href={url} target='_blank' rel='noopener noreferrer'>
            {content}
        </a>
    ) : (
        <span>{content}</span>
    );
};

export const isSHA = (revision: string) => {
    // https://stackoverflow.com/questions/468370/a-regex-to-match-a-sha1
    return revision.match(/^[a-f0-9]{5,40}$/) !== null;
};
