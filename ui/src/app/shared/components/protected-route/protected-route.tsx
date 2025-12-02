import {DataLoader} from 'argo-ui';
import * as React from 'react';
import {Redirect, Route, RouteComponentProps, RouteProps} from 'react-router';

import {services} from '../../services';

interface ProtectedRouteProps extends RouteProps {
    component: React.ComponentType<RouteComponentProps<any>>;
    renderWithLayout?: (component: React.ReactElement) => React.ReactElement;
}

async function isExpiredSSO(): Promise<boolean> {
    try {
        // Combine both async calls into one Promise.all for better performance
        const [userInfo, authSettings] = await Promise.all([services.users.get(), services.authService.settings()]);

        if (userInfo.iss && userInfo.iss !== 'argocd') {
            return ((authSettings.dexConfig && authSettings.dexConfig.connectors) || []).length > 0 || !!authSettings.oidcConfig;
        }
    } catch (err) {
        console.error('Failed to check SSO configuration:', err);
        return false;
    }
    return false;
}

export const ProtectedRoute: React.FC<ProtectedRouteProps> = ({component: Component, renderWithLayout, ...rest}) => {
    return (
        <Route
            {...rest}
            render={routeProps => {
                // Never protect the login route - render it directly without auth check
                if (routeProps.location.pathname === '/login') {
                    return <Component {...routeProps} />;
                }

                // Use pathname only in cache key to avoid re-checking auth on query param changes
                // This prevents conflicts when filters update URL query params
                const cacheKey = routeProps.location.pathname;

                return (
                    <DataLoader
                        input={cacheKey}
                        load={async () => {
                            try {
                                const userInfo = await services.users.get();
                                return {loggedIn: userInfo.loggedIn};
                            } catch (err: any) {
                                // If request fails with 401, user is not logged in
                                if (err?.response?.status === 401 || err?.status === 401) {
                                    return {loggedIn: false};
                                }
                                // For other errors, throw to let error handler deal with it
                                throw err;
                            }
                        }}>
                        {({loggedIn}) => {
                            if (!loggedIn) {
                                // Prevent redirect loop - if already on login page, just render nothing
                                if (routeProps.location.pathname === '/login') {
                                    return null;
                                }
                                // User is not logged in, check if SSO is configured
                                const currentPath = routeProps.location.pathname + routeProps.location.search;
                                const returnUrl = encodeURIComponent(currentPath);

                                return (
                                    <DataLoader
                                        load={async () => {
                                            try {
                                                return await isExpiredSSO();
                                            } catch (err) {
                                                console.error('Failed to check SSO expiration status:', err);
                                                return false;
                                            }
                                        }}>
                                        {isSSO => {
                                            if (isSSO) {
                                                const basehref = document.querySelector('head > base')?.getAttribute('href')?.replace(/\/$/, '') || '';
                                                window.location.href = `${basehref}/auth/login?return_url=${returnUrl}`;
                                                return null;
                                            } else {
                                                return <Redirect to={`/login?return_url=${returnUrl}`} />;
                                            }
                                        }}
                                    </DataLoader>
                                );
                            }

                            // User is logged in, render with layout
                            if (renderWithLayout) {
                                return renderWithLayout(<Component {...routeProps} />);
                            }

                            return <Component {...routeProps} />;
                        }}
                    </DataLoader>
                );
            }}
        />
    );
};
