import * as React from 'react';

require('./banner.scss');

interface Banner {
    icon: BannerIcon;
    style: string;
}

export enum BannerIcon {
    Info = 'fas fa-info-circle',
    Warning = 'fas fa-exclamation-triangle',
    Error = 'fas fa-exclamation-circle'
}

export enum BannerType {
    Info = 'info',
    Warning = 'warning',
    Error = 'error'
}

export function Banner(type: BannerType, icon: BannerIcon, content: string) {
    return (
        <div className={`banner banner__${type}`}>
            <i className={icon} />
            {content}
        </div>
    );
}
