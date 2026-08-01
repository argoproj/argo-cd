import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import * as React from 'react';
import { GlobalDiffModal } from './GlobalDiffModal';
import { ApplicationDiffAccordion } from './ApplicationDiffAccordion';
import * as models from '../../../shared/models';
import { services } from '../../../shared/services';

jest.mock('../../../shared/services', () => ({
    services: {
        applications: {
            getBatchApplicationDiff: jest.fn(),
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

const mockDiffResponses = [
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
];

describe('GlobalDiffModal & ApplicationDiffAccordion', () => {
    beforeEach(() => {
        jest.clearAllMocks();
        (services.applications.getBatchApplicationDiff as jest.Mock).mockImplementation(() =>
            Promise.resolve(mockDiffResponses)
        );
    });

    test('renders GlobalDiffModal and verifies stats, search filtering, and collapsible accordion behavior', async () => {
        const handleClose = jest.fn();

        render(<GlobalDiffModal isShown={true} onClose={handleClose} allApps={mockAllApps} />);

        await waitFor(() => {
            expect(screen.getByText('app-1')).toBeInTheDocument();
        });

        expect(screen.getByText('app-2')).toBeInTheDocument();

        expect(screen.getByText('2')).toBeInTheDocument();

        const searchInput = screen.getByPlaceholderText('Filter by Kind, Namespace, or Application Name...');
        fireEvent.change(searchInput, { target: { value: 'app-1' } });
        expect(screen.getByText('app-1')).toBeInTheDocument();
        expect(screen.queryByText('app-2')).not.toBeInTheDocument();

        const accordionHeader = screen.getByText('app-1').closest('.application-diff-accordion__header');
        expect(screen.getByTestId('resources-diff')).toBeInTheDocument();
        fireEvent.click(accordionHeader);
        expect(screen.queryByTestId('resources-diff')).not.toBeInTheDocument();
    });

    test('verifies Expand All and Collapse All buttons functionality', async () => {
        render(<GlobalDiffModal isShown={true} onClose={() => {}} allApps={mockAllApps} />);

        await waitFor(() => {
            expect(screen.getByText('app-1')).toBeInTheDocument();
        });

        // 1. By default, app-1 diff is visible (expanded by default)
        expect(screen.getByTestId('resources-diff')).toBeInTheDocument();

        // 2. Click Collapse All
        const collapseAllBtn = screen.getByText('Collapse All');
        fireEvent.click(collapseAllBtn);

        // app-1 diff should be collapsed and hidden
        expect(screen.queryByTestId('resources-diff')).not.toBeInTheDocument();

        // 3. Click Expand All
        const expandAllBtn = screen.getByText('Expand All');
        fireEvent.click(expandAllBtn);

        // app-1 diff should be expanded and visible again
        expect(screen.getByTestId('resources-diff')).toBeInTheDocument();
    });

    test('verifies empty state when no apps are out of sync', async () => {
        render(<GlobalDiffModal isShown={true} onClose={() => {}} allApps={[]} />);
        await waitFor(() => {
            expect(screen.getByText(/No applications with out-of-sync or drifted resources found/i)).toBeInTheDocument();
        });
    });

    test('verifies Sync All Selected triggers sync callbacks for out-of-sync apps', async () => {
        render(<GlobalDiffModal isShown={true} onClose={() => {}} allApps={mockAllApps} />);

        await waitFor(() => {
            expect(screen.getByText('app-1')).toBeInTheDocument();
        });

        const syncAllBtn = screen.getByText('Sync All Selected');
        fireEvent.click(syncAllBtn);

        await waitFor(() => {
            expect(services.applications.sync).toHaveBeenCalledTimes(2);
            expect(services.applications.sync).toHaveBeenCalledWith('app-1', 'argocd', null, false, false, null, null);
            expect(services.applications.sync).toHaveBeenCalledWith('app-2', 'argocd', null, false, false, null, null);
        });
    });

    test('verifies lazy loading functionality on ApplicationDiffAccordion', async () => {
        const handleExpand = jest.fn(() => Promise.resolve());
        const mockLazySummary = {
            appName: 'app-lazy',
            project: 'default',
            syncStatus: 'OutOfSync',
            appNamespace: 'argocd',
            diffs: [],
            isLazy: true
        };

        render(
            <ApplicationDiffAccordion
                appSummary={mockLazySummary}
                isLazy={true}
                isLoadingLazy={false}
                onExpand={handleExpand}
            />
        );

        const accordionHeader = screen.getByText('app-lazy').closest('.application-diff-accordion__header');
        // Since it starts expanded by default, click once to collapse it
        fireEvent.click(accordionHeader);
        // Click again to expand it, triggering onExpand
        fireEvent.click(accordionHeader);

        expect(handleExpand).toHaveBeenCalledTimes(1);
    });

    test('verifies inline Sync button on accordion triggers individual sync', async () => {
        const mockSummary = {
            appName: 'app-sync-test',
            project: 'default',
            syncStatus: 'OutOfSync',
            appNamespace: 'argocd',
            diffs: []
        };

        render(<ApplicationDiffAccordion appSummary={mockSummary} />);

        const syncBtn = screen.getByText('Sync');
        fireEvent.click(syncBtn);

        expect(services.applications.sync).toHaveBeenCalledWith('app-sync-test', 'argocd', null, false, false, null, null);
    });
});
