import * as React from 'react';
import {renderToStaticMarkup} from 'react-dom/server.node';
import {CredentialTemplateStatus} from './repos-list';

test('CredentialTemplateStatus renders a key icon and a clear label instead of a bare dash', () => {
    const markup = renderToStaticMarkup(<CredentialTemplateStatus />);

    expect(markup).toContain('fa-key');
    expect(markup).toContain('Credential template');
    expect(markup).not.toContain('>-<');
    expect(markup).toMatchSnapshot();
});
