import * as PropTypes from 'prop-types';
import * as React from 'react';
import {useTranslation} from 'react-i18next';

import {Page} from '../../../shared/components';
import {AppContext} from '../../../shared/context';
import en from '../../../locales/en';

require('./settings-overview.scss');

const settings = [
    {
        title: 'settings-overview.repositories.title',
        description: 'settings-overview.repositories.description',
        path: './repos'
    },
    {
        title: 'settings-overview.repository-certificates-and-known-hosts.title',
        description: 'settings-overview.repository-certificates-and-known-hosts.description',
        path: './certs'
    },
    {
        title: 'settings-overview.gnupg-keys.title',
        description: 'settings-overview.gnupg-keys.description',
        path: './gpgkeys'
    },
    {
        title: 'settings-overview.clusters.title',
        description: 'settings-overview.clusters.description',
        path: './clusters'
    },
    {
        title: 'settings-overview.projects.title',
        description: 'settings-overview.projects.description',
        path: './projects'
    },
    {
        title: 'settings-overview.accounts.title',
        description: 'settings-overview.accounts.description',
        path: './accounts'
    },
    {
        title: 'settings-overview.appearance.title',
        description: 'settings-overview.appearance.description',
        path: './appearance'
    }
];

export const SettingsOverview: React.StatelessComponent = (props: any, context: AppContext) => {
    const {t} = useTranslation();

    return (
        <Page
            title={t('settings-overview.title', en['settings-overview.title'])}
            toolbar={{breadcrumbs: [{title: t('settings-overview.breadcrumbs.0', en['settings-overview.breadcrumbs.0'])}]}}>
            <div className='settings-overview'>
                <div className='argo-container'>
                    {settings.map(item => (
                        <div key={item.path} className='settings-overview__redirect-panel' onClick={() => context.apis.navigation.goto(item.path)}>
                            <div className='settings-overview__redirect-panel__content'>
                                <div className='settings-overview__redirect-panel__title'>{t(item.title)}</div>
                                <div className='settings-overview__redirect-panel__description'>{t(item.description)}</div>
                            </div>
                            <div className='settings-overview__redirect-panel__arrow'>
                                <i className='fa fa-angle-right' />
                            </div>
                        </div>
                    ))}
                </div>
            </div>
        </Page>
    );
};

SettingsOverview.contextTypes = {
    apis: PropTypes.object
};
