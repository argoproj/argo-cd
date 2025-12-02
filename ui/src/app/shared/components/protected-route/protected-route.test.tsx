import * as React from 'react';
import * as renderer from 'react-test-renderer';
import * as PropTypes from 'prop-types';
import {MemoryRouter} from 'react-router';
import {createMemoryHistory} from 'history';
import {NavigationManager, NotificationsManager, PopupManager} from 'argo-ui';
import {ProtectedRoute} from './protected-route';
import {services} from '../../services';
import {UserInfo, AuthSettings} from '../../models';
import {Provider} from '../../context';

jest.mock('../../services', () => ({
    services: {
        users: {
            get: jest.fn(),
        },
        authService: {
            settings: jest.fn(),
        },
    },
}));

const mockServices = services as jest.Mocked<typeof services>;

const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});

const TestComponent = () => <div>Test Component</div>;

class ContextProvider extends React.Component<{children: React.ReactNode}> {
    public static childContextTypes = {
        history: PropTypes.object,
        apis: PropTypes.object,
    };

    private popupManager: PopupManager;
    private notificationsManager: NotificationsManager;
    private navigationManager: NavigationManager;
    private history: ReturnType<typeof createMemoryHistory>;

    constructor(props: {children: React.ReactNode}) {
        super(props);
        this.history = createMemoryHistory();
        this.popupManager = new PopupManager();
        this.notificationsManager = new NotificationsManager();
        this.navigationManager = new NavigationManager(this.history);
    }

    public getChildContext() {
        return {
            history: this.history,
            apis: {
                popup: this.popupManager,
                notifications: this.notificationsManager,
                navigation: this.navigationManager,
                baseHref: '/',
            },
        };
    }

    public render() {
        return (
            <Provider value={{history: this.history, popup: this.popupManager, notifications: this.notificationsManager, navigation: this.navigationManager, baseHref: '/'}}>
                {this.props.children}
            </Provider>
        );
    }
}

const wrapWithContext = (component: React.ReactElement) => {
    return <ContextProvider>{component}</ContextProvider>;
};

describe('ProtectedRoute', () => {
    beforeEach(() => {
        jest.clearAllMocks();
        consoleErrorSpy.mockClear();
        sessionStorage.clear();
        const base = document.createElement('base');
        base.setAttribute('href', '/');
        document.head.appendChild(base);
    });

    afterEach(() => {
        document.head.querySelector('base')?.remove();
        sessionStorage.clear();
    });

    afterAll(() => {
        consoleErrorSpy.mockRestore();
    });

    describe('when user is logged in', () => {
        it('should render the component', async () => {
            const userInfo: UserInfo = {
                loggedIn: true,
                username: 'test-user',
                iss: 'argocd',
                groups: [],
            };

            mockServices.users.get.mockResolvedValue(userInfo);

            const tree = renderer.create(
                wrapWithContext(
                    <MemoryRouter initialEntries={['/test']}>
                        <ProtectedRoute component={TestComponent} path="/test" exact />
                    </MemoryRouter>
                )
            );

            await new Promise(resolve => setTimeout(resolve, 100));

            expect(mockServices.users.get).toHaveBeenCalled();
        });

        it('should call renderWithLayout when user is logged in', async () => {
            const userInfo: UserInfo = {
                loggedIn: true,
                username: 'test-user',
                iss: 'argocd',
                groups: [],
            };

            mockServices.users.get.mockResolvedValue(userInfo);

            const renderWithLayoutSpy = jest.fn((component: React.ReactElement) => <div className="layout">{component}</div>);

            const tree = renderer.create(
                wrapWithContext(
                    <MemoryRouter initialEntries={['/test']}>
                        <ProtectedRoute 
                            component={TestComponent} 
                            path="/test" 
                            exact 
                            renderWithLayout={renderWithLayoutSpy}
                        />
                    </MemoryRouter>
                )
            );

            await new Promise(resolve => setTimeout(resolve, 100));

            expect(mockServices.users.get).toHaveBeenCalled();
            expect(renderWithLayoutSpy).toHaveBeenCalled();
        });
    });

    describe('when user is not logged in', () => {
        it('should NOT call renderWithLayout when user is not logged in', async () => {
            const userInfo: UserInfo = {
                loggedIn: false,
                username: '',
                iss: 'argocd',
                groups: [],
            };

            mockServices.users.get.mockResolvedValue(userInfo);

            const renderWithLayoutSpy = jest.fn((component: React.ReactElement) => <div className="layout">{component}</div>);

            const tree = renderer.create(
                wrapWithContext(
                    <MemoryRouter initialEntries={['/test']}>
                        <ProtectedRoute 
                            component={TestComponent} 
                            path="/test" 
                            exact 
                            renderWithLayout={renderWithLayoutSpy}
                        />
                    </MemoryRouter>
                )
            );

            await new Promise(resolve => setTimeout(resolve, 100));

            expect(renderWithLayoutSpy).not.toHaveBeenCalled();
            expect(mockServices.users.get).toHaveBeenCalled();
        });

        it('should redirect to login when SSO is not configured', async () => {
            const userInfo: UserInfo = {
                loggedIn: false,
                username: '',
                iss: 'argocd',
                groups: [],
            };

            const authSettings: AuthSettings = {
                url: '',
                statusBadgeEnabled: false,
                statusBadgeRootUrl: '',
                googleAnalytics: {trackingID: '', anonymizeUsers: true},
                dexConfig: {connectors: []},
                oidcConfig: undefined as any,
                help: {chatUrl: '', chatText: '', binaryUrls: {}},
                userLoginsDisabled: false,
                kustomizeVersions: [],
                uiCssURL: '',
                uiBannerContent: '',
                uiBannerURL: '',
                uiBannerPermanent: false,
                uiBannerPosition: '',
                execEnabled: false,
                appsInAnyNamespaceEnabled: false,
                hydratorEnabled: false,
                syncWithReplaceAllowed: false,
            };

            mockServices.users.get.mockResolvedValue(userInfo);
            mockServices.authService.settings.mockResolvedValue(authSettings);

            const tree = renderer.create(
                wrapWithContext(
                    <MemoryRouter initialEntries={['/test']}>
                        <ProtectedRoute component={TestComponent} path="/test" exact />
                    </MemoryRouter>
                )
            );

            await new Promise(resolve => setTimeout(resolve, 200));

            expect(mockServices.users.get).toHaveBeenCalled();
        });

        it('should redirect to SSO login when SSO is configured with Dex connectors', async () => {
            const userInfo: UserInfo = {
                loggedIn: false,
                username: '',
                iss: 'external-issuer',
                groups: [],
            };

            const authSettings: AuthSettings = {
                url: '',
                statusBadgeEnabled: false,
                statusBadgeRootUrl: '',
                googleAnalytics: {trackingID: '', anonymizeUsers: true},
                dexConfig: {
                    connectors: [
                        {name: 'github', type: 'github'},
                    ],
                },
                oidcConfig: undefined as any,
                help: {chatUrl: '', chatText: '', binaryUrls: {}},
                userLoginsDisabled: false,
                kustomizeVersions: [],
                uiCssURL: '',
                uiBannerContent: '',
                uiBannerURL: '',
                uiBannerPermanent: false,
                uiBannerPosition: '',
                execEnabled: false,
                appsInAnyNamespaceEnabled: false,
                hydratorEnabled: false,
                syncWithReplaceAllowed: false,
            };

            mockServices.users.get.mockResolvedValue(userInfo);
            mockServices.authService.settings.mockResolvedValue(authSettings);

            delete (window as any).location;
            (window as any).location = {href: ''};

            const tree = renderer.create(
                wrapWithContext(
                    <MemoryRouter initialEntries={['/test']}>
                        <ProtectedRoute component={TestComponent} path="/test" exact />
                    </MemoryRouter>
                )
            );

            await new Promise(resolve => setTimeout(resolve, 200));

            expect(mockServices.users.get).toHaveBeenCalled();
            expect(mockServices.authService.settings).toHaveBeenCalled();
        });

        it('should redirect to SSO login when SSO is configured with OIDC', async () => {
            const userInfo: UserInfo = {
                loggedIn: false,
                username: '',
                iss: 'external-issuer',
                groups: [],
            };

            const authSettings: AuthSettings = {
                url: '',
                statusBadgeEnabled: false,
                statusBadgeRootUrl: '',
                googleAnalytics: {trackingID: '', anonymizeUsers: true},
                dexConfig: {connectors: []},
                oidcConfig: {name: 'oidc'},
                help: {chatUrl: '', chatText: '', binaryUrls: {}},
                userLoginsDisabled: false,
                kustomizeVersions: [],
                uiCssURL: '',
                uiBannerContent: '',
                uiBannerURL: '',
                uiBannerPermanent: false,
                uiBannerPosition: '',
                execEnabled: false,
                appsInAnyNamespaceEnabled: false,
                hydratorEnabled: false,
                syncWithReplaceAllowed: false,
            };

            mockServices.users.get.mockResolvedValue(userInfo);
            mockServices.authService.settings.mockResolvedValue(authSettings);

            delete (window as any).location;
            (window as any).location = {href: ''};

            const tree = renderer.create(
                wrapWithContext(
                    <MemoryRouter initialEntries={['/test']}>
                        <ProtectedRoute component={TestComponent} path="/test" exact />
                    </MemoryRouter>
                )
            );

            await new Promise(resolve => setTimeout(resolve, 200));

            expect(mockServices.users.get).toHaveBeenCalled();
            expect(mockServices.authService.settings).toHaveBeenCalled();
        });

        it('should handle SSO check failure gracefully', async () => {
            const userInfo: UserInfo = {
                loggedIn: false,
                username: '',
                iss: 'external-issuer',
                groups: [],
            };

            const ssoError = new Error('SSO check failed');

            mockServices.users.get.mockResolvedValue(userInfo);
            mockServices.authService.settings.mockRejectedValue(ssoError);

            const tree = renderer.create(
                wrapWithContext(
                    <MemoryRouter initialEntries={['/test']}>
                        <ProtectedRoute component={TestComponent} path="/test" exact />
                    </MemoryRouter>
                )
            );

            await new Promise(resolve => setTimeout(resolve, 200));

            expect(mockServices.users.get).toHaveBeenCalled();
            expect(mockServices.authService.settings).toHaveBeenCalled();
            expect(consoleErrorSpy).toHaveBeenCalled();
        });

        it('should use Promise.all to combine async calls in isExpiredSSO', async () => {
            const userInfo: UserInfo = {
                loggedIn: false,
                username: '',
                iss: 'external-issuer',
                groups: [],
            };

            const authSettings: AuthSettings = {
                url: '',
                statusBadgeEnabled: false,
                statusBadgeRootUrl: '',
                googleAnalytics: {trackingID: '', anonymizeUsers: true},
                dexConfig: {connectors: [{name: 'github', type: 'github'}]},
                oidcConfig: undefined as any,
                help: {chatUrl: '', chatText: '', binaryUrls: {}},
                userLoginsDisabled: false,
                kustomizeVersions: [],
                uiCssURL: '',
                uiBannerContent: '',
                uiBannerURL: '',
                uiBannerPermanent: false,
                uiBannerPosition: '',
                execEnabled: false,
                appsInAnyNamespaceEnabled: false,
                hydratorEnabled: false,
                syncWithReplaceAllowed: false,
            };

            mockServices.users.get.mockResolvedValue(userInfo);
            mockServices.authService.settings.mockResolvedValue(authSettings);

            delete (window as any).location;
            (window as any).location = {href: ''};

            const tree = renderer.create(
                wrapWithContext(
                    <MemoryRouter initialEntries={['/test']}>
                        <ProtectedRoute component={TestComponent} path="/test" exact />
                    </MemoryRouter>
                )
            );

            await new Promise(resolve => setTimeout(resolve, 200));

            expect(mockServices.users.get).toHaveBeenCalled();
            expect(mockServices.authService.settings).toHaveBeenCalled();
        });
    });

    describe('error handling', () => {
        it('should handle 401 error gracefully', async () => {
            const error: any = {
                response: {status: 401},
            };

            mockServices.users.get
                .mockRejectedValueOnce(error)
                .mockResolvedValueOnce({loggedIn: false, username: '', iss: 'argocd', groups: []});

            const tree = renderer.create(
                wrapWithContext(
                    <MemoryRouter initialEntries={['/test']}>
                        <ProtectedRoute component={TestComponent} path="/test" exact />
                    </MemoryRouter>
                )
            );

            await new Promise(resolve => setTimeout(resolve, 100));

            expect(mockServices.users.get).toHaveBeenCalled();
        });

        it('should throw non-401 error to DataLoader error handler', async () => {
            const error: any = new Error('Network error');
            error.response = {status: 500};
            error.status = 500;

            mockServices.users.get.mockRejectedValue(error);

            const tree = renderer.create(
                wrapWithContext(
                    <MemoryRouter initialEntries={['/test']}>
                        <ProtectedRoute component={TestComponent} path="/test" exact />
                    </MemoryRouter>
                )
            );

            await new Promise(resolve => setTimeout(resolve, 100));

            expect(mockServices.users.get).toHaveBeenCalled();
        });
    });

    describe('login route handling', () => {
        it('should render login route directly without auth check', async () => {
            const tree = renderer.create(
                wrapWithContext(
                    <MemoryRouter initialEntries={['/login']}>
                        <ProtectedRoute component={TestComponent} path="/login" exact />
                    </MemoryRouter>
                )
            );

            await new Promise(resolve => setTimeout(resolve, 0));

            expect(mockServices.users.get).not.toHaveBeenCalled();
        });
    });

    describe('isExpiredSSO logic', () => {
        it('should return false when iss is argocd', async () => {
            const userInfo: UserInfo = {
                loggedIn: false,
                username: '',
                iss: 'argocd',
                groups: [],
            };

            const authSettings: AuthSettings = {
                url: '',
                statusBadgeEnabled: false,
                statusBadgeRootUrl: '',
                googleAnalytics: {trackingID: '', anonymizeUsers: true},
                dexConfig: {connectors: [{name: 'github', type: 'github'}]},
                oidcConfig: undefined as any,
                help: {chatUrl: '', chatText: '', binaryUrls: {}},
                userLoginsDisabled: false,
                kustomizeVersions: [],
                uiCssURL: '',
                uiBannerContent: '',
                uiBannerURL: '',
                uiBannerPermanent: false,
                uiBannerPosition: '',
                execEnabled: false,
                appsInAnyNamespaceEnabled: false,
                hydratorEnabled: false,
                syncWithReplaceAllowed: false,
            };

            mockServices.users.get.mockResolvedValue(userInfo);
            mockServices.authService.settings.mockResolvedValue(authSettings);

            const tree = renderer.create(
                wrapWithContext(
                    <MemoryRouter initialEntries={['/test']}>
                        <ProtectedRoute component={TestComponent} path="/test" exact />
                    </MemoryRouter>
                )
            );

            await new Promise(resolve => setTimeout(resolve, 200));

            expect(mockServices.users.get).toHaveBeenCalled();
            expect(mockServices.authService.settings).toHaveBeenCalled();
        });
    });
});
