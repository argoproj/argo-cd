import * as React from 'react';
import {repoUrl} from './urls';

export const Repo = ({url, children}: {url: string; children?: React.ReactNode}) => {
    const href = repoUrl(url);
    const content = children || url;
    return href !== null ? (
        <a href={href} target='_blank'>
            {content}
        </a>
    ) : (
        <span>{content}</span>
    );
};
