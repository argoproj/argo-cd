import {DataLoader, Page as ArgoPage, Toolbar, Utils} from 'argo-ui';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import {BehaviorSubject, Observable} from 'rxjs';
import {AppContext} from '../context';
import {services} from '../services';

const mostRecentLoggedIn = new BehaviorSubject<boolean>(false);

function isLoggedIn(): Observable<boolean> {
    services.users.get().then(info => mostRecentLoggedIn.next(info.loggedIn));
    return mostRecentLoggedIn;
}

export class Page extends React.Component<{title: string; toolbar?: Toolbar | Observable<Toolbar>}> {
    public static contextTypes = {
        router: PropTypes.object,
        history: PropTypes.object
    };

    public render() {
        return (
            <DataLoader
                input={new Date()}
                load={() =>
                    Utils.toObservable(this.props.toolbar).map(toolbar => {
                        toolbar = toolbar || {};
                        toolbar.tools = [
                            toolbar.tools,
                            <DataLoader key='loginPanel' load={() => isLoggedIn()}>
                                {loggedIn =>
                                    loggedIn ? (
                                        <a key='logout' onClick={() => this.goToLogin(true)}>
                                            Log out
                                        </a>
                                    ) : (
                                        <a key='login' onClick={() => this.goToLogin(false)}>
                                            Log in
                                        </a>
                                    )
                                }
                            </DataLoader>
                        ];
                        return toolbar;
                    })
                }>
                {toolbar => <ArgoPage title={this.props.title} children={this.props.children} toolbar={toolbar} />}
            </DataLoader>
        );
    }

    private async goToLogin(logout = false) {
        if (logout) {
            await services.users.logout();
        }
        this.appContext.history.push('/login');
    }

    private get appContext(): AppContext {
        return this.context as AppContext;
    }
}
