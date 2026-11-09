import {DropDownMenu} from 'argo-ui';
import * as React from 'react';
import {isValidURL} from '../../shared/utils';

// Ingress regex paths are not directly navigable. Keep the usable prefix before the first regex group.
export const getExternalURL = (url: string): string => url.split('(')[0];

export class InvalidExternalLinkError extends Error {
    constructor(message: string) {
        super(message);
        Object.setPrototypeOf(this, InvalidExternalLinkError.prototype);
        this.name = 'InvalidExternalLinkError';
    }
}

export class ExternalLink {
    public title: string;
    public ref: string;

    constructor(url: string) {
        // A raw URL may contain pipe characters in its path (for example, an
        // Ingress path using a regular expression). Only interpret a pipe as
        // the title/URL separator when the complete value is not itself a URL.
        const isAbsoluteHttpUrl = /^https?:\/\//i.test(url);
        const parts = isAbsoluteHttpUrl ? [url] : url.split('|');
        const title = parts.length === 2 ? parts[0] : url;
        const ref = parts.length === 2 ? parts[1] : url;
        this.ref = getExternalURL(ref);
        this.title = isAbsoluteHttpUrl || parts.length !== 2 ? this.ref : title;
        if (!isValidURL(this.ref)) {
            throw new InvalidExternalLinkError('Invalid URL');
        }
    }
}

export const ExternalLinks = (urls?: string[]) => {
    const externalLinks: ExternalLink[] = [];
    for (const url of urls || []) {
        try {
            const externalLink = new ExternalLink(url);
            externalLinks.push(externalLink);
        } catch {
            continue;
        }
    }

    // sorted alphabetically & links with titles first
    externalLinks.sort((a, b) => {
        const hasTitle = (x: ExternalLink): boolean => {
            return x.title !== x.ref && x.title !== '';
        };

        if (hasTitle(a) && hasTitle(b) && a.title !== b.title) {
            return a.title > b.title ? 1 : -1;
        } else if (hasTitle(b) && !hasTitle(a)) {
            return 1;
        } else if (hasTitle(a) && !hasTitle(b)) {
            return -1;
        }
        return a.ref > b.ref ? 1 : -1;
    });

    return externalLinks;
};

export const ApplicationURLs = ({urls}: {urls: string[]}) => {
    const externalLinks: ExternalLink[] = ExternalLinks(urls);

    return (
        ((externalLinks || []).length > 0 && (
            <div className='applications-list__external-links-icon-container'>
                {externalLinks.length === 1 ? (
                    <a href={externalLinks[0].ref} title={externalLinks[0].title} target='_blank' rel='noopener noreferrer' onClick={e => e.stopPropagation()}>
                        <i className='fa fa-external-link-alt' />
                    </a>
                ) : (
                    <DropDownMenu
                        anchor={() => (
                            <a title='External URLs'>
                                <i className='fa fa-share-square' />
                            </a>
                        )}
                        items={externalLinks.map(item => ({
                            title: item.title,
                            action: () => window.open(item.ref, '_blank', 'noopener,noreferrer')
                        }))}
                    />
                )}
            </div>
        )) ||
        null
    );
};
