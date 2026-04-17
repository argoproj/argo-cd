import * as React from 'react';
import * as renderer from 'react-test-renderer';

import {CREDENTIALS_COLUMN_LABEL, CREDENTIAL_TEMPLATE_STATUS_LABEL, CredentialTemplateStatus} from './repos-list.helpers';

test('credential column label is expanded', () => {
    expect(CREDENTIALS_COLUMN_LABEL).toBe('Credentials');
});

test('CredentialTemplateStatus renders a key icon and clear status text', () => {
    const root = renderer.create(<CredentialTemplateStatus />).root;

    expect(root.findByType('i').props.className).toContain('fa fa-key');
    expect(root.findByType('span').children.join('')).toContain(CREDENTIAL_TEMPLATE_STATUS_LABEL);
});
