import React from 'react';
import {act, fireEvent, render, screen} from '@testing-library/react';
import {LoginFactory} from './login';
import {AuthSettings} from '../../shared/models';
import {MinimalHistory, Provider} from '../../shared/context';
import {NotificationsManager, PopupManager} from 'argo-ui';
import {LocationState, Path} from 'history';

const settings: AuthSettings = {
    appsInAnyNamespaceEnabled: false,
    dexConfig: {
        connectors: [
            {
                name: 'string',
                type: 'string'
            }
        ]
    },
    execEnabled: false,
    googleAnalytics: {
        trackingID: 'foo',
        anonymizeUsers: true
    },
    help: {
        chatUrl: '',
        chatText: '',
        binaryUrls: {}
    },
    kustomizeVersions: [],
    oidcConfig: {
        name: ''
    },
    statusBadgeEnabled: true,
    statusBadgeRootUrl: '',
    uiBannerContent: '',
    uiBannerPermanent: true,
    uiBannerPosition: '',
    uiBannerURL: '',
    uiCssURL: '',
    url: '',
    userLoginsDisabled: false
};

const signIn = 'Sign In';
const history: MinimalHistory = {
    location: {
        pathname: '',
        search: '',
        state: '',
        hash: '',
        key: ''
    },
    push(path: Path, state?: LocationState) {
        // no-op
    }
};

test('loads and displays login page (and can do a happy-path submission)', async () => {
    const Login = LoginFactory(
        () => Promise.resolve(settings),
        (userName, password) => {
            expect(userName).toEqual('some user');
            expect(password).toEqual('some password');
            return Promise.resolve({token: 'some token'});
        }
    );

    const expectedPaths = ['/applications', '.'];
    const expectedQueries: {[p: string]: any}[] = [undefined, {sso_error: null}];

    await act(async () =>
        render(
            <Provider
                value={{
                    history,
                    popup: new PopupManager(),
                    notifications: new NotificationsManager(),
                    navigation: {
                        goto(path: string, query?: {[p: string]: any}, options?: {event?: React.MouseEvent; replace?: boolean}) {
                            const expectedPath = expectedPaths.pop();
                            const expectedQuery = expectedQueries.pop();
                            expect(path).toEqual(expectedPath);
                            expect(query).toEqual(expectedQuery);
                        }
                    },
                    baseHref: 'my-base.com'
                }}>
                <Login history={history} />
            </Provider>
        )
    );

    const userNameRequired = 'Username is required';
    const passwordRequired = 'Password is required';

    expect(screen.queryByText(userNameRequired)).toBeNull();
    expect(screen.queryByText(passwordRequired)).toBeNull();

    await act(async () => fireEvent.click(screen.getByText(signIn)));

    expect(screen.queryByText(userNameRequired)).not.toBeNull();
    expect(screen.queryByText(passwordRequired)).not.toBeNull();

    fireEvent.change(screen.getByLabelText('Username'), {target: {value: 'some user'}});

    await act(async () => fireEvent.click(screen.getByText(signIn)));

    expect(screen.queryByText(userNameRequired)).toBeNull();
    expect(screen.queryByText(passwordRequired)).not.toBeNull();

    fireEvent.change(screen.getByLabelText('Password'), {target: {value: 'some password'}});

    await act(async () => fireEvent.click(screen.getByText(signIn)));

    expect(screen.queryByText(userNameRequired)).toBeNull();
    expect(screen.queryByText(passwordRequired)).toBeNull();
});

test('does not display login form if disabled in settings', async () => {
    const Login = LoginFactory(
        () => Promise.resolve({...settings, userLoginsDisabled: true, dexConfig: undefined, oidcConfig: undefined}),
        (userName, password) => {
            return Promise.resolve({token: 'some token'});
        }
    );

    await act(async () =>
        render(
            <Provider
                value={{
                    history,
                    popup: new PopupManager(),
                    notifications: new NotificationsManager(),
                    navigation: {
                        goto(path: string, query?: {[p: string]: any}, options?: {event?: React.MouseEvent; replace?: boolean}) {
                            // no-op
                        }
                    },
                    baseHref: 'my-base.com'
                }}>
                <Login history={history} />
            </Provider>
        )
    );

    expect(screen.queryByText(signIn)).toBeNull();
    expect(screen.queryByText('Login is disabled. Please contact your system administrator.')).not.toBeNull();
});

test('displays sso login if enabled in settings', async () => {
    const Login = LoginFactory(
        () =>
            Promise.resolve({
                ...settings,
                userLoginsDisabled: true,
                oidcConfig: {
                    name: 'your OIDC Provider'
                }
            }),
        (userName, password) => {
            return Promise.resolve({token: 'some token'});
        }
    );

    await act(async () =>
        render(
            <Provider
                value={{
                    history,
                    popup: new PopupManager(),
                    notifications: new NotificationsManager(),
                    navigation: {
                        goto(path: string, query?: {[p: string]: any}, options?: {event?: React.MouseEvent; replace?: boolean}) {
                            // no-op
                        }
                    },
                    baseHref: 'my-base.com'
                }}>
                <Login history={history} />
            </Provider>
        )
    );

    expect(screen.queryByText(signIn)).toBeNull();
    expect(screen.queryByText('Log in via your OIDC Provider')).not.toBeNull();
    expect(screen.queryByText('Login is disabled. Please contact your system administrator.')).toBeNull();
});

test('displays sso login error if redirected', async () => {
    const Login = LoginFactory(
        () =>
            Promise.resolve({
                ...settings,
                userLoginsDisabled: true,
                oidcConfig: {
                    name: 'your OIDC Provider'
                }
            }),
        (userName, password) => {
            return Promise.resolve({token: 'some token'});
        }
    );

    await act(async () =>
        render(
            <Provider
                value={{
                    history,
                    popup: new PopupManager(),
                    notifications: new NotificationsManager(),
                    navigation: {
                        goto(path: string, query?: {[p: string]: any}, options?: {event?: React.MouseEvent; replace?: boolean}) {
                            // no-op
                        }
                    },
                    baseHref: 'my-base.com'
                }}>
                <Login
                    history={{
                        ...history,
                        location: {
                            search: 'has_sso_error=true',
                            pathname: '',
                            state: '',
                            hash: '',
                            key: ''
                        }
                    }}
                />
            </Provider>
        )
    );

    expect(screen.queryByText(signIn)).toBeNull();
    expect(screen.queryByText('Login failed.')).not.toBeNull();
    expect(screen.queryByText('Log in via your OIDC Provider')).not.toBeNull();
    expect(screen.queryByText('Login is disabled. Please contact your system administrator.')).toBeNull();
});
