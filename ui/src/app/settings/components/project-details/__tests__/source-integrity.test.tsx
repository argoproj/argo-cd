import * as React from 'react';
import {render, screen} from '@testing-library/react';

jest.mock('../../../../applications/components/utils', () => ({
    helpTip: (text: string) => React.createElement('span', {title: text}, '?')
}));

import {SourceIntegrityTab} from '../source-integrity';
import {Project, SourceIntegrityGitPolicy} from '../../../../shared/models';
import {Context} from '../../../../shared/context';

const emptyProject = {
    spec: {
        sourceIntegrity: undefined
    }
} as unknown as Project;

const mockContext = {
    history: {} as any,
    popup: {} as any,
    navigation: {} as any,
    baseHref: '/',
    notifications: {
        show: jest.fn()
    } as any
};

function projectWithGitPolicies(policies: SourceIntegrityGitPolicy[]): Project {
    return {
        spec: {
            sourceIntegrity: {
                git: {policies}
            }
        }
    } as unknown as Project;
}

function renderTab(proj: Project) {
    return render(
        <Context.Provider value={mockContext}>
            <SourceIntegrityTab proj={proj} loadSignatureKeys={jest.fn()} saveProject={jest.fn()} />
        </Context.Provider>
    );
}

describe('SourceIntegrityTab', () => {
    test('shows empty state when source integrity is unset', () => {
        renderTab(emptyProject);

        expect(screen.getByText(/SOURCE INTEGRITY/)).toBeTruthy();
        expect(screen.getByText('Source Integrity is not configured.')).toBeTruthy();
        expect(screen.getByText(/configured via CLI or manifests/)).toBeTruthy();
        expect(screen.queryByText('GIT')).toBeFalsy();
    });

    test('renders a git GPG policy with keys, included and excluded repos', () => {
        renderTab(
            projectWithGitPolicies([
                {
                    repos: [{url: 'https://github.com/example/app.git'}, {url: '!https://github.com/example/skip.git'}],
                    gpg: {mode: 'head', keys: ['ABCD1234', 'EFGH5678']}
                }
            ])
        );

        expect(screen.getByText('GIT')).toBeTruthy();
        expect(screen.getByText('MODE')).toBeTruthy();
        expect(screen.getByText('head')).toBeTruthy();
        expect(screen.getByText('KEYS')).toBeTruthy();
        expect(screen.getByText('ABCD1234, EFGH5678')).toBeTruthy();
        expect(screen.getAllByText('REPO-URLS')).toHaveLength(1);
        expect(screen.getByText('https://github.com/example/app.git')).toBeTruthy();
        expect(screen.getAllByText('EXCLUDED REPO-URLS')).toHaveLength(1);
        expect(screen.getByText('https://github.com/example/skip.git')).toBeTruthy();
        expect(screen.queryByText('!https://github.com/example/skip.git')).toBeFalsy();
        expect(screen.queryByText('Source Integrity is not configured.')).toBeFalsy();
    });

    test('shows None when a policy has only excluded repo URLs', () => {
        renderTab(
            projectWithGitPolicies([
                {
                    repos: [{url: '!https://github.com/example/skip.git'}],
                    gpg: {mode: 'head', keys: ['ABCD1234']}
                }
            ])
        );

        expect(screen.getByText('GIT')).toBeTruthy();
        expect(screen.getAllByText('REPO-URLS')).toHaveLength(1);
        expect(screen.getAllByText('EXCLUDED REPO-URLS')).toHaveLength(1);
        expect(screen.getByText('None')).toBeTruthy();
        expect(screen.getByText('https://github.com/example/skip.git')).toBeTruthy();
        expect(screen.queryByText('!https://github.com/example/skip.git')).toBeFalsy();
    });

    test('renders two git GPG policies', () => {
        renderTab(
            projectWithGitPolicies([
                {
                    repos: [{url: 'https://github.com/example/app.git'}],
                    gpg: {mode: 'head', keys: ['ABCD1234']}
                },
                {
                    repos: [{url: 'https://github.com/example/other.git'}],
                    gpg: {mode: 'strict', keys: ['EFGH5678']}
                }
            ])
        );

        expect(screen.getByText('GIT')).toBeTruthy();
        expect(screen.getAllByText('REPO-URLS')).toHaveLength(2);
        expect(screen.queryByText('EXCLUDED REPO-URLS')).toBeFalsy();
        expect(screen.getByText('head')).toBeTruthy();
        expect(screen.getByText('ABCD1234')).toBeTruthy();
        expect(screen.getByText('https://github.com/example/app.git')).toBeTruthy();
        expect(screen.getByText('strict')).toBeTruthy();
        expect(screen.getByText('EFGH5678')).toBeTruthy();
        expect(screen.getByText('https://github.com/example/other.git')).toBeTruthy();
    });

    test('shows empty deprecated GPG signature keys panel', () => {
        renderTab(emptyProject);

        expect(screen.getByText(/\[DEPRECATED\] GPG SIGNATURE KEYS/)).toBeTruthy();
        expect(screen.getByText('This feature is deprecated, migrate to Source Integrity instead.')).toBeTruthy();
        expect(screen.getByText('Project has no signature keys')).toBeTruthy();
    });

    test('lists deprecated GPG signature keys', () => {
        const project = {
            spec: {
                signatureKeys: [{keyID: '1234567890'}, {keyID: '1234567891'}]
            }
        } as unknown as Project;

        renderTab(project);

        expect(screen.getByText(/\[DEPRECATED\] GPG SIGNATURE KEYS/)).toBeTruthy();
        expect(screen.getByText('This feature is deprecated, migrate to Source Integrity instead.')).toBeTruthy();
        expect(screen.getByText('1234567890')).toBeTruthy();
        expect(screen.getByText('1234567891')).toBeTruthy();
        expect(screen.queryByText('Project has no signature keys')).toBeFalsy();
    });
});
