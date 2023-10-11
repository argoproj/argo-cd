import {DataLoader, DropDownMenu} from 'argo-ui';
import * as React from 'react';

import {services} from '../../../shared/services';
import {Context} from '../../context';

require('./badge-panel.scss');

export const BadgePanel = ({app, project}: {app?: string; project?: string}) => {
    const [badgeType, setBadgeType] = React.useState('URL');
    const context = React.useContext(Context);
    if (!app && !project) {
        throw new Error('Either app or project property must be specified');
    }

    function badgeContent(statusBadgeRootUrl: string) {
        const root = statusBadgeRootUrl ? statusBadgeRootUrl : `${location.protocol}//${location.host}${context.baseHref}`;
        let badgeURL = '';
        let entityURL = '';
        let alt = '';
        if (app) {
            badgeURL = `${root}api/badge?name=${app}&revision=true&lastSyncTime=false`;
            entityURL = `${root}applications/${app}`;
            alt = 'App Status';
        } else if (project) {
            badgeURL = `${root}api/badge?project=${project}&revision=true&lastSyncTime=false`;
            entityURL = `${root}projects/${project}`;
            alt = 'Project Status';
        }
        return (
            <div className='white-box'>
                <div className='white-box__details'>
                    <p>STATUS BADGE</p>
                    <p>
                        <img src={badgeURL} />{' '}
                    </p>
                    <div className='white-box__details-row'>
                        <DropDownMenu
                            anchor={() => (
                                <p>
                                    {badgeType} <i className='fa fa-caret-down' />
                                </p>
                            )}
                            items={['URL', 'Markdown', 'Textile', 'Rdoc', 'AsciiDoc'].map(type => ({title: type, action: () => setBadgeType(type)}))}
                        />
                        <textarea
                            onClick={e => (e.target as HTMLInputElement).select()}
                            className='badge-panel'
                            readOnly={true}
                            value={
                                badgeType === 'URL'
                                    ? badgeURL
                                    : badgeType === 'Markdown'
                                    ? `[![${alt}](${badgeURL})](${entityURL})`
                                    : badgeType === 'Textile'
                                    ? `!${badgeURL}!:${entityURL}`
                                    : badgeType === 'Rdoc'
                                    ? `{<img src="${badgeURL}" alt="${alt}" />}[${entityURL}]`
                                    : badgeType === 'AsciiDoc'
                                    ? `image:${badgeURL}["${alt}", link="${entityURL}"]`
                                    : ''
                            }
                        />
                    </div>
                </div>
            </div>
        );
    }

    return (
        <DataLoader load={() => services.authService.settings()}>
            {settings => (settings.statusBadgeEnabled && <div>{badgeContent(settings.statusBadgeRootUrl)}</div>) || null}
        </DataLoader>
    );
};
