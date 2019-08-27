import {DataLoader, Page as ArgoPage, Toolbar, Utils} from 'argo-ui';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import {Observable} from 'rxjs';
import {Session} from '../models';
import {AppContext} from '../context';
import {services} from '../services';


export class Page extends React.Component<{ title: string, toolbar?: Toolbar | Observable<Toolbar> }> {
    public static contextTypes = {
        router: PropTypes.object,
        history: PropTypes.object,
    };

    public render() {
        return (
            <DataLoader input={new Date()}
                        load={() => Observable.zip(Utils.toObservable(this.props.toolbar), services.users.get()).map(([toolbar, account]: [Toolbar, Session]) => {
                toolbar = toolbar || {};
                toolbar.tools = [
                    toolbar.tools,
                    account.username ?
                        <a key='logout' onClick={() => this.goToLogin(true)}>Logout</a> :
                        <a key='login' onClick={() => this.goToLogin(false)}>Login</a>,
                ];
                return toolbar;
            })}>
            {(toolbar) => (
                <ArgoPage title={this.props.title} children={this.props.children} toolbar={toolbar} />
            )}
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
