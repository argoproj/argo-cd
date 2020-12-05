import {DropDownMenu} from 'argo-ui';
import * as React from 'react';

class ExternalLink {
    public title: string;
    public ref: string;

    constructor(url: string) {
        const parts = url.split('|');
        if (parts.length === 2) {
            this.title = parts[0];
            this.ref = parts[1];
        } else {
            this.title = url;
            this.ref = url;
        }
    }
}

export const ApplicationURLs = ({urls}: {urls: string[]}) => {
    const externalLinks: ExternalLink[] = [];
    for (const url of urls || []) {
        externalLinks.push(new ExternalLink(url));
    }

    // sorted alphabetically & links with titles first
    externalLinks.sort((a, b) => {
        if (a.title !== '' && b.title !== '') {
            return a.title > b.title ? 1 : -1;
        } else if (a.title === '') {
            return 1;
        } else if (b.title === '') {
            return -1;
        }
        return a.ref > b.ref ? 1 : -1;
    });

    return (
        ((externalLinks || []).length > 0 && (
            <span>
                <a
                    title={externalLinks[0].title}
                    onClick={e => {
                        e.stopPropagation();
                        window.open(externalLinks[0].ref);
                    }}>
                    <i className='fa fa-external-link-alt' />{' '}
                    {externalLinks.length > 1 && (
                        <DropDownMenu
                            anchor={() => <i className='fa fa-caret-down' />}
                            items={externalLinks.map(item => ({
                                title: item.title,
                                action: () => window.open(item.ref)
                            }))}
                        />
                    )}
                </a>
            </span>
        )) ||
        null
    );
};
