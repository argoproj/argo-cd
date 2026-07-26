import {DataLoader, Page as ArgoPage, Toolbar} from 'argo-ui';
import * as React from 'react';
import {BehaviorSubject, Observable} from 'rxjs';

import {services} from '../../services';
import requests from '../../services/requests';

const mostRecentLoggedIn = new BehaviorSubject<boolean>(false);

import './page.scss';

function isLoggedIn(): Observable<boolean> {
    services.users.get().then(info => mostRecentLoggedIn.next(info.loggedIn));
    return mostRecentLoggedIn;
}

export const AuthOption: React.FC = () => {
    return (
        <DataLoader load={() => isLoggedIn()}>
            {loggedIn =>
                loggedIn ? (
                    <button className='login-logout-button' onClick={() => (window.location.href = requests.toAbsURL('/auth/logout'))}>
                        Log out
                    </button>
                ) : (
                    <button className='login-logout-button' onClick={() => (window.location.href = `/login?return_url=${encodeURIComponent(location.href)}`)}>
                        Log in
                    </button>
                )
            }
        </DataLoader>
    );
};

interface PageProps {
    title: string;
    toolbar?: Toolbar | Observable<Toolbar>;
    topBarTitle?: string;
    useTitleOnly?: boolean;
    children?: React.ReactNode;
}

// Check if toolbar has content (tools or actionMenu)
const hasToolbarContent = (toolbar: Toolbar | Observable<Toolbar> | undefined): boolean => {
    if (!toolbar) {
        return false;
    }

    // If it's an Observable, we can't check synchronously, so assume it has content
    if (toolbar instanceof Observable) {
        return true;
    }

    // Check if toolbar has tools or actionMenu with items
    return !!(toolbar.tools || (toolbar.actionMenu && toolbar.actionMenu.items && toolbar.actionMenu.items.length > 0));
};

export const Page = (props: PageProps) => {
    const applyPageWrapper = !hasToolbarContent(props.toolbar);

    return (
        <DataLoader load={() => services.viewPreferences.getPreferences()}>
            {pref => (
                <div className={`${applyPageWrapper ? 'page-wrapper' : ''} ${pref.hideSidebar ? 'sb-page-wrapper__sidebar-collapsed' : 'sb-page-wrapper'}`}>
                    <ArgoPage title={props.title} children={props.children} topBarTitle={props.topBarTitle} useTitleOnly={props.useTitleOnly} toolbar={props.toolbar} />
                </div>
            )}
        </DataLoader>
    );
};
