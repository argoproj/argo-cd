import * as React from 'react';
import {act, render, screen} from '@testing-library/react';
import '@testing-library/jest-dom';
import {DataLoader} from 'argo-ui';
import {BehaviorSubject} from 'rxjs';

import {PageSuspense} from './page-suspense';

describe('PageSuspense', () => {
    it('keeps the layout following preference changes after a lazy page loads', async () => {
        const prefs = new BehaviorSubject({hideSidebar: false});
        let resolvePage: (page: {default: React.ComponentType}) => void;
        const LazyPage = React.lazy(() => new Promise<{default: React.ComponentType}>(resolve => (resolvePage = resolve)));

        // Same shape as the app: the layout reads the preferences through a DataLoader and renders the lazy page inside it.
        render(
            <DataLoader load={() => prefs}>
                {pref => (
                    <div data-testid='layout' data-collapsed={String(pref.hideSidebar)}>
                        <PageSuspense>
                            <LazyPage />
                        </PageSuspense>
                    </div>
                )}
            </DataLoader>
        );
        expect(screen.getByTestId('layout')).toHaveAttribute('data-collapsed', 'false');

        await act(async () => resolvePage({default: () => <div>page</div>}));
        expect(await screen.findByText('page')).toBeInTheDocument();

        act(() => prefs.next({hideSidebar: true}));
        expect(screen.getByTestId('layout')).toHaveAttribute('data-collapsed', 'true');
    });
});
