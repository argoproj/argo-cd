import * as PropTypes from 'prop-types';
import * as React from 'react';
import {t} from 'i18next';

import {Page} from '../../../shared/components';
import {AppContext} from '../../../shared/context';
import en from '../../../locales/en';

require('./settings-overview.scss');

const settings = [
    {
        title: t('settings-overview.repositories.title', en['settings-overview.repositories.title']),
        description: t('settings-overview.repositories.description', en['settings-overview.repositories.description']),
        path: './repos'
    },
    {
        title: t('settings-overview.repository-certificates-and-known-hosts.title', en['settings-overview.repository-certificates-and-known-hosts.title']),
        description: t('settings-overview.repository-certificates-and-known-hosts.description', en['settings-overview.repository-certificates-and-known-hosts.description']),
        path: './certs'
    },
    {
        title: t('settings-overview.gnupg-keys.title', en['settings-overview.gnupg-keys.title']),
        description: t('settings-overview.gnupg-keys.description', en['settings-overview.gnupg-keys.description']),
        path: './gpgkeys'
    },
    {
        title: t('settings-overview.clusters.title', en['settings-overview.clusters.title']),
        description: t('settings-overview.clusters.description', en['settings-overview.clusters.description']),
        path: './clusters'
    },
    {
        title: t('settings-overview.projects.title', en['settings-overview.projects.title']),
        description: t('settings-overview.projects.description', en['settings-overview.projects.description']),
        path: './projects'
    },
    {
        title: t('settings-overview.accounts.title', en['settings-overview.accounts.title']),
        description: t('settings-overview.accounts.description', en['settings-overview.accounts.description']),
        path: './accounts'
    },
    {
        title: t('settings-overview.appearance.title', en['settings-overview.appearance.title']),
        description: t('settings-overview.appearance.description', en['settings-overview.appearance.description']),
        path: './appearance'
    }
];

export const SettingsOverview: React.StatelessComponent = (props: any, context: AppContext) => (
    <Page title={t('settings-overview.title', en['settings-overview.title'])} toolbar={{breadcrumbs: [{title: t('settings-overview.breadcrumbs.0', en['settings-overview.breadcrumbs.0'])}]}}>
        <div className='settings-overview'>
            <div className='argo-container'>
                {settings.map(item => (
                    <div key={item.path} className='settings-overview__redirect-panel' onClick={() => context.apis.navigation.goto(item.path)}>
                        <div className='settings-overview__redirect-panel__content'>
                            <div className='settings-overview__redirect-panel__title'>{item.title}</div>
                            <div className='settings-overview__redirect-panel__description'>{item.description}</div>
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

SettingsOverview.contextTypes = {
    apis: PropTypes.object
};
