import * as React from 'react';
import {render} from '@testing-library/react';
import {RevisionMetadataRows, truncateRevisionMessage} from './revision-metadata-rows';

jest.mock('argo-ui', () => ({
    DataLoader: ({input, load, children}: {input: unknown; load: (input: unknown) => Promise<unknown>; children: (data: unknown) => React.ReactNode}) => {
        const [data, setData] = React.useState<unknown>(null);
        React.useEffect(() => {
            let cancelled = false;
            load(input).then(result => {
                if (!cancelled) {
                    setData(result);
                }
            });
            return () => {
                cancelled = true;
            };
        }, []);
        return data ? <>{children(data)}</> : null;
    },
    Tooltip: ({content, children}: {content: React.ReactNode; children: React.ReactNode}) => (
        <div data-testid='tooltip-wrapper'>
            <div data-testid='tooltip'>{content}</div>
            <div data-testid='preview'>{children}</div>
        </div>
    )
}));

jest.mock('../../../shared/components', () => ({
    Timestamp: () => null
}));

jest.mock('../../../shared/services', () => ({
    services: {
        applications: {
            revisionMetadata: jest.fn(() =>
                Promise.resolve({
                    author: 'Test Author',
                    date: '2026-01-01T00:00:00Z',
                    message: 'update: from release-hotfix-fix-transcript-format-9b5de948 to release-v2.10.8-25124b65',
                    signatureInfo: ''
                })
            ),
            revisionChartDetails: jest.fn(),
            ociMetadata: jest.fn()
        }
    }
}));

describe('truncateRevisionMessage', () => {
    it('returns short first lines unchanged', () => {
        expect(truncateRevisionMessage('build: bump', 64)).toBe('build: bump');
    });

    it('uses only the first line', () => {
        expect(truncateRevisionMessage('line1\nline2', 64)).toBe('line1');
    });

    it('caps the preview length', () => {
        const message = 'x'.repeat(200);
        expect(truncateRevisionMessage(message, 64)).toHaveLength(64);
    });

    it('handles empty message', () => {
        expect(truncateRevisionMessage('', 64)).toBe('');
    });
});

describe('RevisionMetadataRows', () => {
    const gitSource = {repoURL: 'https://example.com/repo.git', targetRevision: 'abc123', path: '.'};

    it('shows a 64-char first-line preview and the full raw message in the tooltip', async () => {
        const message = 'update: from release-hotfix-fix-transcript-format-9b5de948 to release-v2.10.8-25124b65';
        const {findByTestId} = render(<RevisionMetadataRows applicationName='test-app' applicationNamespace='argocd' source={gitSource as any} index={0} versionId={1} />);

        expect((await findByTestId('preview')).textContent).toBe(message.slice(0, 64));
        expect((await findByTestId('tooltip')).textContent).toBe(message);
        expect(message.length).toBeGreaterThan(64);
    });

    it('preserves multi-line commit message bodies in the tooltip', async () => {
        const {services} = require('../../../shared/services');
        const message = 'short subject\n\nLonger body explaining the change in detail.';
        services.applications.revisionMetadata.mockResolvedValueOnce({
            author: 'Test Author',
            date: '2026-01-01T00:00:00Z',
            message,
            signatureInfo: ''
        });
        const {findByTestId} = render(<RevisionMetadataRows applicationName='test-app' applicationNamespace='argocd' source={gitSource as any} index={2} versionId={3} />);

        expect((await findByTestId('preview')).textContent).toBe('short subject');
        expect((await findByTestId('tooltip')).textContent).toBe(message);
    });

    it('skips the Tooltip entirely for short, single-line commit messages', async () => {
        const {services} = require('../../../shared/services');
        const message = 'fix: typo';
        services.applications.revisionMetadata.mockResolvedValueOnce({
            author: 'Test Author',
            date: '2026-01-01T00:00:00Z',
            message,
            signatureInfo: ''
        });
        const {findByText, queryByTestId} = render(
            <RevisionMetadataRows applicationName='test-app' applicationNamespace='argocd' source={gitSource as any} index={3} versionId={4} />
        );

        expect(await findByText(message)).toBeTruthy();
        expect(queryByTestId('tooltip-wrapper')).toBeNull();
    });

    it('calls revisionMetadata for git sources', async () => {
        const {services} = require('../../../shared/services');
        services.applications.revisionMetadata.mockClear();
        const {findByTestId} = render(<RevisionMetadataRows applicationName='test-app' applicationNamespace='argocd' source={gitSource as any} index={1} versionId={2} />);
        await findByTestId('preview');
        expect(services.applications.revisionMetadata).toHaveBeenCalledWith('test-app', 'argocd', 'abc123', 1, 2);
    });
});
