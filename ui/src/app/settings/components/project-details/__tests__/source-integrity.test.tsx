import * as React from 'react';
import {render, screen} from '@testing-library/react';
import '@testing-library/jest-dom';

jest.mock('../../../../applications/components/utils', () => ({
    helpTip: (text: string) => React.createElement('span', {title: text}, '?')
}));

import {SourceIntegrityTab} from '../source-integrity';
import {Project, ProjectSourceIntegrity, SourceIntegrityGitPolicy} from '../../../../shared/models';
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

function projectWithSignatureKeys(keyIDs: string[], sourceIntegrity?: ProjectSourceIntegrity | undefined): Project {
    return {
        spec: {
            signatureKeys: keyIDs.map(keyID => ({keyID})),
            sourceIntegrity
        }
    } as unknown as Project;
}

function renderTab(proj: Project) {
    return render(
        <Context.Provider value={mockContext}>
            <SourceIntegrityTab proj={proj} saveProject={jest.fn()} />
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

    test('shows None when a policy has no keys', () => {
        renderTab(projectWithGitPolicies([{repos: [{url: 'https://github.com/example/app.git'}], gpg: {mode: 'head', keys: []}}]));

        expect(screen.getByText('GIT')).toBeTruthy();
        expect(screen.getAllByText('KEYS')).toHaveLength(1);
        expect(screen.getByText('None')).toBeTruthy();
        expect(screen.getAllByText('REPO-URLS')).toHaveLength(1);
        expect(screen.getByText('https://github.com/example/app.git')).toBeTruthy();
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
        expect(screen.getByText('This feature is deprecated. Migrate to Source Integrity instead.')).toBeTruthy();
        expect(screen.getByText('Project has no signature keys')).toBeTruthy();
        expect(screen.queryByText(/not used for sync/)).toBeFalsy();
        expect(screen.queryByText(/still enforced/)).toBeFalsy();
    });

    test('lists deprecated GPG signature keys and explains they are still enforced', () => {
        renderTab(projectWithSignatureKeys(['1234567890', '1234567891']));

        expect(screen.getByText(/\[DEPRECATED\] GPG SIGNATURE KEYS/)).toBeTruthy();
        expect(screen.getByText('This feature is deprecated. Migrate to Source Integrity instead.')).toBeTruthy();
        expect(screen.getByText('1234567890')).toBeTruthy();
        expect(screen.getByText('1234567891')).toBeTruthy();
        expect(screen.queryByText('Project has no signature keys')).toBeFalsy();
        expect(screen.getByText(/still enforced for all Git repositories/)).toBeTruthy();
        expect(screen.getByText(/same as a Source Integrity Git policy in head mode/)).toBeTruthy();
        expect(screen.getByRole('link', {name: 'Upgrade to Source Integrity'})).toHaveAttribute(
            'href',
            'https://argo-cd.readthedocs.io/en/stable/user-guide/source-integrity-git-gpg/#upgrade-to-source-integrity-verification'
        );
        expect(screen.queryByText(/not used for sync/)).toBeFalsy();
    });

    test('explains signature keys are not used for sync when Source Integrity Git policies are configured', () => {
        renderTab(
            projectWithSignatureKeys(['1234567890'], {
                git: {
                    policies: [
                        {
                            repos: [{url: 'https://github.com/example/app.git'}],
                            gpg: {mode: 'strict', keys: ['ABCD1234']}
                        }
                    ]
                }
            })
        );

        expect(screen.getByText('1234567890')).toBeTruthy();
        expect(screen.getByText(/not used for sync/)).toBeTruthy();
        expect(screen.getByText(/because Source Integrity Git policies are configured/)).toBeTruthy();
        expect(screen.getByRole('link', {name: 'Upgrade to Source Integrity'})).toHaveAttribute(
            'href',
            'https://argo-cd.readthedocs.io/en/stable/user-guide/source-integrity-git-gpg/#upgrade-to-source-integrity-verification'
        );
        expect(screen.queryByText(/still enforced/)).toBeFalsy();
    });
});
