import * as React from 'react';
import {isValidURL} from '../utils';

export const ValidatedLink = ({url, children, ...anchorProps}: Omit<React.AnchorHTMLAttributes<HTMLAnchorElement>, 'href'> & {url: string}) => {
    if (!isValidURL(url)) {
        return null;
    }

    return (
        <a {...anchorProps} href={url}>
            {children}
        </a>
    );
};
