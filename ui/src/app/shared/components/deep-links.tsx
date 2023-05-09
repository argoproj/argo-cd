import * as React from 'react';
import {LinkInfo} from '../models';

export const DeepLinks = (props: {links: LinkInfo[]}) => {
    const {links} = props;
    return (
        <div style={{margin: '10px 0'}}>
            {(links || []).map((link: LinkInfo) => (
                <div key={link.title} style={{display: 'flex', alignItems: 'center', height: '35px'}}>
                    <a href={link.url} target='_blank' style={{display: 'flex', alignItems: 'center', marginRight: '7px'}} rel='noopener'>
                        <i className={`fa ${link.iconClass ? link.iconClass : 'fa-external-link-alt'}`} style={{marginRight: '5px'}} />
                        <div>{link.title}</div>
                    </a>
                    {link.description && <>({link.description})</>}
                </div>
            ))}
        </div>
    );
};
