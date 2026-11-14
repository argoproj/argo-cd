import React from 'react';
import {Link} from 'react-router-dom';
import {LinkInfo} from '../models';

export const DeepLinks = (props: {links: LinkInfo[]}) => {
    const {links} = props;
    return (
        <div style={{margin: '10px 0'}}>
            {(links || []).map((link: LinkInfo) => (
                <div key={link.title} style={{display: 'flex', alignItems: 'center', height: '35px'}}>
                    {link.url.startsWith('http') ? (
                        <a href={link.url} target='_blank' rel='noopener' style={{display: 'flex', alignItems: 'center', marginRight: '7px'}}>
                            <i className={`fa ${link.iconClass ? link.iconClass : 'fa-external-link-alt'} custom-style-link`} style={{marginRight: '5px'}} />
                            <div>{link.title}</div>
                        </a>
                    ) : (
                        <Link to={link.url} style={{display: 'flex', alignItems: 'center', marginRight: '7px'}}>
                            <i className={`fa ${link.iconClass ? link.iconClass : 'fa-external-link-alt'}`} style={{marginRight: '5px'}} />
                            <div>{link.title}</div>
                        </Link>
                    )}
                    {link.description && <>({link.description})</>}
                </div>
            ))}
        </div>
    );
};
