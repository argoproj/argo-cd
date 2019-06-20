import * as React from 'react';
import {repoUrl} from './urls';

export const Repo = ({url, children}: { url: string, children?: React.ReactNode }) => {
    url = repoUrl(url);
    const content = children || url;
    return url !== null ? <a href={url}>{content}</a> : <span>{content}</span>;
};
