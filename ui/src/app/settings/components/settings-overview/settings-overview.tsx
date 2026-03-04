import * as React from 'react';

import {Page} from '../../../shared/components';
import {Context} from '../../../shared/context';

require('./settings-overview.scss');

const settings = [
    {
        title: 'Repositories',
        description: 'Configure connected repositories',
        path: './repos'
    },
    {
        title: 'Repository certificates and known hosts',
        description: 'Configure repository certificates and known hosts for connecting Git repositories',
        path: './certs'
    },
    {
        title: 'GnuPG keys',
        description: 'Configure GnuPG public keys for commit verification',
        path: './gpgkeys'
    },
    {
        title: 'Clusters',
        description: 'Configure connected Kubernetes clusters',
        path: './clusters'
    },
    {
        title: 'Projects',
        description: 'Configure Argo CD projects',
        path: './projects'
    },
    {
        title: 'Accounts',
        description: 'Configure Accounts',
        path: './accounts'
    },
    {
        title: 'Appearance',
        description: 'Configure themes in UI',
        path: './appearance'
    }
];

export const SettingsOverview: React.FC = () => {
    const context = React.useContext(Context);
    return (
        <Page title='Settings' toolbar={{breadcrumbs: [{title: 'Settings'}]}}>
            <div className='settings-overview'>
                <div className='argo-container'>
                    {settings.map(item => (
                        <div key={item.path} className='settings-overview__redirect-panel' onClick={() => context.navigation.goto(item.path)}>
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
};
