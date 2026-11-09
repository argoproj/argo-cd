import * as React from 'react';

import {ExternalLink} from '../application-urls';

export const ApplicationSummaryExternalURLs = ({urls}: {urls: string[]}) => {
    const links = urls.reduce((result, url) => {
        try {
            result.push(new ExternalLink(url));
        } catch {
            // Ignore invalid or unsafe external URLs.
        }
        return result;
    }, [] as ExternalLink[]);

    return (
        <div className='application-summary__links-rows'>
            {links.map((link, i) => (
                <div className='application-summary__links-row' key={i}>
                    <a href={link.ref} target='_blank' rel='noopener noreferrer'>
                        {link.title} &nbsp;
                    </a>
                </div>
            ))}
        </div>
    );
};
