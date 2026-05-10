import * as React from 'react';

import {Repo} from '../../../shared/components';

export const CREDENTIALS_COLUMN_LABEL = 'Credentials';
export const CREDENTIAL_TEMPLATE_STATUS_LABEL = 'Configured';

export const CredentialTemplateUrl = ({url}: {url: string}) => (
    <div className='repos-list__credential-template-url'>
        <i className='icon argo-icon-git' />
        <Repo url={url} />
    </div>
);

export const CredentialTemplateStatus = () => (
    <span className='repos-list__credential-template-status'>
        <i className='fa fa-key' /> {CREDENTIAL_TEMPLATE_STATUS_LABEL}
    </span>
);
