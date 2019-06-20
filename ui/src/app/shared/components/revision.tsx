import * as React from 'react';
import {revisionUrl} from './urls';

export const Revision = ({repoUrl, revision, children}: { repoUrl: string, revision: string, children?: React.ReactNode }) => {
    revision = revision || 'HEAD';
    const url = revisionUrl(repoUrl, revision);
    const content = children || revision.substr(0, 7);
    return url !== null ? <a href={url}>{content}</a> : <span>{content}</span>;
};
