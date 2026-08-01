import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import * as React from 'react';
import { GlobalDiffModal } from './GlobalDiffModal';
import * as models from '../../../shared/models';

jest.mock('../../../shared/services', () => ({
    services: {
        applications: {
            getBatchApplicationDiff: jest.fn(() =>
                Promise.resolve([
                    {
                        appName: 'app-1',
                        project: 'default',
                        syncStatus: 'OutOfSync',
                        appNamespace: 'argocd',
                        diffs: [
                            {
                                kind: 'Deployment',
                                name: 'my-deploy',
                                namespace: 'default',
                                liveState: '{}',
                                targetState: '{}'
                            }
                        ]
                    },
                    {
                        appName: 'app-2',
                        project: 'default',
                        syncStatus: 'OutOfSync',
                        appNamespace: 'argocd',
                        diffs: []
                    }
                ])
            ),
            sync: jest.fn(() => Promise.resolve(true))
        }
    }
}));

jest.mock('../application-resources-diff/application-resources-diff', () => ({
    ApplicationResourcesDiff: () => <div data-testid='resources-diff'>Resources Diff Content</div>
}));

jest.mock('../utils', () => ({
    ComparisonStatusIcon: ({ status }: { status: string }) => <div data-testid={`status-${status}`}>{status}</div>
}));

const mockAllApps = [
    {
        metadata: { name: 'app-1', namespace: 'argocd' },
        spec: { project: 'default' },
        status: { sync: { status: 'OutOfSync' }, health: { status: 'Healthy' } }
    },
    {
        metadata: { name: 'app-2', namespace: 'argocd' },
        spec: { project: 'default' },
        status: { sync: { status: 'OutOfSync' }, health: { status: 'Healthy' } }
    }
] as models.Application[];

describe('GlobalDiffModal & ApplicationDiffAccordion', () => {
    test('renders GlobalDiffModal and verifies stats, filtering, and collapsible accordion behavior', async () => {
        const handleClose = jest.fn();

        render(<GlobalDiffModal isShown={true} onClose={handleClose} allApps={mockAllApps} />);

        await waitFor(() => {
            expect(screen.getByText('app-1')).toBeInTheDocument();
        });

        expect(screen.getByText('app-2')).toBeInTheDocument();

        const accordionHeader = screen.getByText('app-1').closest('.application-diff-accordion__header');
        expect(screen.queryByTestId('resources-diff')).not.toBeInTheDocument();

        fireEvent.click(accordionHeader);
        expect(screen.getByTestId('resources-diff')).toBeInTheDocument();

        fireEvent.click(accordionHeader);
        expect(screen.queryByTestId('resources-diff')).not.toBeInTheDocument();
    });

    test('verifies search query filters applications in the modal', async () => {
        render(<GlobalDiffModal isShown={true} onClose={() => {}} allApps={mockAllApps} />);

        await waitFor(() => {
            expect(screen.getByText('app-1')).toBeInTheDocument();
        });

        const searchInput = screen.getByPlaceholderText('Filter by Kind, Namespace, or Application Name...');

        fireEvent.change(searchInput, { target: { value: 'app-1' } });
        expect(screen.getByText('app-1')).toBeInTheDocument();
        expect(screen.queryByText('app-2')).not.toBeInTheDocument();
    });
});
