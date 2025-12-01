import {DataLoader} from 'argo-ui';
import * as React from 'react';
import {Redirect, Route, RouteComponentProps, RouteProps} from 'react-router';

import {services} from '../../services';

interface ProtectedRouteProps extends RouteProps {
    component: React.ComponentType<RouteComponentProps<any>>;
    noLayout?: boolean;
    renderWithLayout?: (component: React.ReactElement) => React.ReactElement;
}

async function isExpiredSSO(): Promise<boolean> {
    try {
        const {iss} = await services.users.get();
        const authSettings = await services.authService.settings();
        if (iss && iss !== 'argocd') {
            return ((authSettings.dexConfig && authSettings.dexConfig.connectors) || []).length > 0 || !!authSettings.oidcConfig;
        }
    } catch {
        return false;
    }
    return false;
}

export const ProtectedRoute: React.FC<ProtectedRouteProps> = ({component: Component, noLayout, renderWithLayout, ...rest}) => {
    return (
        <Route
            {...rest}
            render={routeProps => {
                return (
                    <DataLoader
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
                                // User is not logged in, redirect to login immediately
                                const currentPath = routeProps.location.pathname + routeProps.location.search;
                                const returnUrl = encodeURIComponent(currentPath);

                                // Check if SSO is configured
                                return (
                                    <DataLoader
                                        load={async () => {
                                            try {
                                                return await isExpiredSSO();
                                            } catch {
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

                            // User is logged in, render the component
                            if (noLayout) {
                                return <Component {...routeProps} />;
                            }

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
