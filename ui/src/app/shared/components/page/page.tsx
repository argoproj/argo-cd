import {DataLoader, Page as ArgoPage, Toolbar, Utils} from 'argo-ui';
import * as React from 'react';
import {BehaviorSubject, Observable} from 'rxjs';

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
    return Utils.toObservable(init).map(toolbar => {
        toolbar = toolbar || {};
        toolbar.tools = [
            toolbar.tools,
            <DataLoader key='loginPanel' load={() => isLoggedIn()}>
                {loggedIn =>
                    loggedIn ? (
                        <a key='logout' onClick={() => (window.location.href = requests.toAbsURL('/auth/logout'))}>
                            Log out
                        </a>
                    ) : (
                        <a key='login' onClick={() => ctx.navigation.goto('/login')}>
                            Log in
                        </a>
                    )
                }
            </DataLoader>
        ];
        return toolbar;
    });
};

interface PageProps extends React.Props<any> {
    title: string;
    hideAuth?: boolean;
    toolbar?: Toolbar | Observable<Toolbar>;
}

export const Page = (props: PageProps) => {
    const ctx = React.useContext(Context);
    return (
        <div className={`${props.hideAuth ? 'page-wrapper' : ''}`}>
            <ArgoPage title={props.title} children={props.children} toolbar={!props.hideAuth ? AddAuthToToolbar(props.toolbar, ctx) : props.toolbar} />
        </div>
    );
};
