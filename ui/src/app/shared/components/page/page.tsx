import {DataLoader, Page as ArgoPage, Toolbar, Utils} from 'argo-ui';
import * as React from 'react';
import {BehaviorSubject, Observable} from 'rxjs';
import {map} from 'rxjs/operators';

import {Context, ContextApis} from '../../context';
import {services} from '../../services';
import requests from '../../services/requests';

const mostRecentLoggedIn = new BehaviorSubject<boolean>(false);

import './page.scss';

function isLoggedIn(): Observable<boolean> {
    services.users.get().then(info => mostRecentLoggedIn.next(info.loggedIn));
    return mostRecentLoggedIn;
}

export const AddAuthToToolbar = (init: Toolbar | Observable<Toolbar>, ctx: ContextApis): Observable<Toolbar> => {
    return Utils.toObservable(init).pipe(
        map(toolbar => {
            toolbar = toolbar || {};
            toolbar.tools = [
                toolbar.tools,
                <DataLoader key='loginPanel' load={() => isLoggedIn()}>
                    {loggedIn =>
                        loggedIn ? (
                            <button className='login-logout-button' key='logout' onClick={() => (window.location.href = requests.toAbsURL('/auth/logout'))}>
                                Log out
                            </button>
                        ) : (
                            <button className='login-logout-button' key='login' onClick={() => ctx.navigation.goto(`/login?return_url=${encodeURIComponent(location.href)}`)}>
                                Log in
                            </button>
                        )
                    }
                </DataLoader>
            ];
            return toolbar;
        })
    );
};

interface PageProps extends React.Props<any> {
    title: string;
    hideAuth?: boolean;
    toolbar?: Toolbar | Observable<Toolbar>;
    topBarTitle?: string;
    useTitleOnly?: boolean;
}

export const Page = (props: PageProps) => {
    const ctx = React.useContext(Context);
    return (
        <DataLoader load={() => services.viewPreferences.getPreferences()}>
            {pref => (
                <div className={`${props.hideAuth ? 'page-wrapper' : ''} ${pref.hideSidebar ? 'sb-page-wrapper__sidebar-collapsed' : 'sb-page-wrapper'}`}>
                    <ArgoPage
                        title={props.title}
                        children={props.children}
                        topBarTitle={props.topBarTitle}
                        useTitleOnly={props.useTitleOnly}
                        toolbar={!props.hideAuth ? AddAuthToToolbar(props.toolbar, ctx) : props.toolbar}
                    />
                </div>
            )}
        </DataLoader>
    );
};
