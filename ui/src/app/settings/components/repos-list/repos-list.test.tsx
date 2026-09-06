import * as React from 'react';
import {renderToStaticMarkup} from 'react-dom/server.node';
import {ConnectionStatusCell, CredentialTemplateStatus} from './repos-list';
import {UnifiedRepo} from './repos-filter';
import * as models from '../../../shared/models';

test('CredentialTemplateStatus renders a key icon and a clear label instead of a bare dash', () => {
    const markup = renderToStaticMarkup(<CredentialTemplateStatus />);

    expect(markup).toContain('fa-key');
    expect(markup).toContain('Credential template');
    expect(markup).not.toContain('>-<');
    expect(markup).toMatchSnapshot();
});

describe('ConnectionStatusCell (the CONNECTION STATUS cell rendered by each ReposList row)', () => {
    test('a credential template with no connection state shows the key icon and label, not a bare dash', () => {
        const templateRepo: UnifiedRepo = {readCred: {url: 'https://github.com/example/repo.git'} as models.RepoCreds};

        const markup = renderToStaticMarkup(<ConnectionStatusCell connectionState={undefined} repo={templateRepo} />);

        expect(markup).toContain('fa-key');
        expect(markup).toContain('Credential template');
        expect(markup).not.toContain('>-<');
    });

    test('a real repository with no connection state yet still shows the bare dash', () => {
        const uncheckedRepo: UnifiedRepo = {readRepo: {repo: 'https://github.com/example/repo.git'} as models.Repository};

        const markup = renderToStaticMarkup(<ConnectionStatusCell connectionState={undefined} repo={uncheckedRepo} />);

        expect(markup).toContain('>-<');
        expect(markup).not.toContain('Credential template');
    });

    test('a repository with a known connection state keeps showing that state, unaffected by the template case', () => {
        const connectedRepo: UnifiedRepo = {readRepo: {repo: 'https://github.com/example/repo.git'} as models.Repository};
        const connectionState = {status: models.ConnectionStatuses.Successful, message: ''} as models.ConnectionState;

        const markup = renderToStaticMarkup(<ConnectionStatusCell connectionState={connectionState} repo={connectedRepo} />);

        expect(markup).toContain(models.ConnectionStatuses.Successful);
        expect(markup).not.toContain('Credential template');
        expect(markup).not.toContain('>-<');
    });
});
