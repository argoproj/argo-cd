import * as React from 'react';
import {act, fireEvent, render, screen} from '@testing-library/react';
import {Link} from 'react-router-dom';
import {App, history} from './app';
import {LayoutProps} from './shared/components/layout/layout';
import {SystemLevelExtension} from './shared/services/extensions-service';

jest.mock('./shared/services', () => {
    const {of} = jest.requireActual('rxjs');
    const {ExtensionsService} = jest.requireActual('./shared/services/extensions-service');
    return {
        services: {
            viewPreferences: {init: jest.fn(), getPreferences: () => of({theme: 'light', hideSidebar: false})},
            version: {version: () => Promise.resolve({Version: 'v0.0.0'})},
            extensions: new ExtensionsService(),
            authService: {settings: () => Promise.resolve({resourceViewEnabled: true})},
            users: {get: () => Promise.resolve({loggedIn: true, username: 'admin', iss: 'argocd', groups: []})}
        }
    };
});

jest.mock('./shared/components/layout/layout', () => {
    // React Compiler caches the sidebar's navigation using the navItems reference.
    const Navigation = React.memo(({navItems}: Pick<LayoutProps, 'navItems'>) => (
        <nav>
            {navItems.map(item => (
                <Link key={item.path} to={item.path}>
                    {item.title}
                </Link>
            ))}
        </nav>
    ));
    return {
        Layout: ({navItems, children}: LayoutProps) => (
            <>
                <Navigation navItems={navItems} />
                {children}
            </>
        ),
        ThemeWrapper: ({children}: React.PropsWithChildren) => <>{children}</>
    };
});

jest.mock('./applications', () => ({__esModule: true, default: {component: () => <div>Applications page</div>}}));
jest.mock('./shared/components/version-info/version-info-panel', () => ({VersionPanel: () => null}));
jest.mock('./ui-banner/ui-banner', () => ({Banner: ({children}: React.PropsWithChildren) => <>{children}</>}));

it('updates memoized navigation and preserves routes when system extensions register after mount, including in a batch', async () => {
    history.replace('/applications');
    const app = React.createRef<App>();
    await act(async () => {
        render(<App ref={app} />);
    });
    expect(await screen.findByText('Applications page')).toBeInTheDocument();
    expect(screen.queryByRole('link', {name: 'First extension'})).not.toBeInTheDocument();

    const {extensionsAPI} = window as typeof window & {
        extensionsAPI: {registerSystemLevelExtension: (component: SystemLevelExtension['component'], title: string, path: string, icon: string) => void};
    };
    const originalNavItems = app.current.state.navItems;
    const originalRoutes = app.current.state.routes;
    act(() => {
        extensionsAPI.registerSystemLevelExtension(() => <div>First extension page</div>, 'First extension', '/extension-first', 'fa-flask');
    });

    expect(screen.getByRole('link', {name: 'First extension'})).toHaveAttribute('href', '/extension-first');
    expect(app.current.state.navItems).not.toBe(originalNavItems);
    expect(app.current.state.routes).not.toBe(originalRoutes);
    expect(originalNavItems.some(item => item.path === '/extension-first')).toBe(false);
    expect(originalRoutes['/extension-first']).toBeUndefined();

    const firstNavItems = app.current.state.navItems;
    const firstRoutes = app.current.state.routes;
    act(() => {
        extensionsAPI.registerSystemLevelExtension(() => <div>Second extension page</div>, 'Second extension', '/extension-second', 'fa-cog');
        extensionsAPI.registerSystemLevelExtension(() => <div>Third extension page</div>, 'Third extension', '/extension-third', 'fa-star');
    });

    expect(app.current.state.navItems).toHaveLength(originalNavItems.length + 3);
    expect(firstNavItems).toHaveLength(originalNavItems.length + 1);
    expect(firstRoutes['/extension-second']).toBeUndefined();
    expect(firstRoutes['/extension-third']).toBeUndefined();
    for (const title of ['First', 'Second', 'Third']) {
        fireEvent.click(screen.getByRole('link', {name: `${title} extension`}));
        expect(screen.getByText(`${title} extension page`)).toBeInTheDocument();
    }
    fireEvent.click(screen.getByRole('link', {name: 'Applications'}));
    expect(await screen.findByText('Applications page')).toBeInTheDocument();
});
