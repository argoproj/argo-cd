import {DataLoader} from 'argo-ui';
import * as React from 'react';
import {Redirect, Route, RouteComponentProps, RouteProps} from 'react-router';

import {services} from '../../services';
import {isSSOConfigured} from '../../utils';

interface ProtectedRouteProps extends RouteProps {
    component: React.ComponentType<RouteComponentProps<any>>;
    renderWithLayout?: (component: React.ReactElement) => React.ReactElement;
}

interface AuthState {
    loggedIn: boolean;
    isSSO: boolean;
}

async function checkAuthState(): Promise<AuthState> {
    try {
        const [userInfo, authSettings] = await Promise.all([services.users.get(), services.authService.settings()]);

        const loggedIn = userInfo?.loggedIn ?? false;

        if (loggedIn) {
            return {loggedIn: true, isSSO: false};
        }

        // Check SSO configuration if user is not logged in
        const hasSSO = isSSOConfigured(userInfo, authSettings);

        return {loggedIn: false, isSSO: hasSSO};
    } catch (err: any) {
        if (err?.response?.status === 401 || err?.status === 401) {
            return {loggedIn: false, isSSO: false};
        }
        throw err;
    }
}

function getBaseHref(): string {
    return document.querySelector('head > base')?.getAttribute('href')?.replace(/\/$/, '') || '';
}

export const ProtectedRoute: React.FC<ProtectedRouteProps> = ({component: Component, renderWithLayout, ...rest}) => {
    return (
        <Route
            {...rest}
            render={routeProps => {
                const isLoginRoute = routeProps.location.pathname === '/login';

                if (isLoginRoute) {
                    return <Component {...routeProps} />;
                }

                // Use pathname only in cache key to avoid re-checking auth on query param changes
                const cacheKey = routeProps.location.pathname;

                return (
                    <DataLoader input={cacheKey} load={checkAuthState}>
                        {({loggedIn, isSSO}) => {
                            if (loggedIn) {
                                return renderWithLayout ? renderWithLayout(<Component {...routeProps} />) : <Component {...routeProps} />;
                            }

                            // User is not logged in - redirect to login or SSO
                            const currentPath = routeProps.location.pathname + routeProps.location.search;
                            const returnUrl = encodeURIComponent(currentPath);

                            if (isSSO) {
                                globalThis.location.href = `${getBaseHref()}/auth/login?return_url=${returnUrl}`;
                                return null;
                            }

                            return <Redirect to={`/login?return_url=${returnUrl}`} />;
                        }}
                    </DataLoader>
                );
            }}
        />
    );
};
