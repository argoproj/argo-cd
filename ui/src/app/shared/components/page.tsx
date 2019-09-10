import {DataLoader, Page as ArgoPage, Toolbar, Utils} from 'argo-ui';
import { parse } from 'cookie';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import {Observable} from 'rxjs';
import {AppContext} from '../context';
import {services} from '../services';

export class Page extends React.Component<{ title: string, toolbar?: Toolbar | Observable<Toolbar> }> {
    public static contextTypes = {
        router: PropTypes.object,
        history: PropTypes.object,
    };

    public render() {
        return (
            <DataLoader input={new Date()} load={() => Utils.toObservable(this.props.toolbar).map((toolbar) => {
                toolbar = toolbar || {};
                toolbar.tools = [
                    toolbar.tools,
                    // this is a crummy check, as the token maybe expired, but it is better than flashing user interface
                    parse(document.cookie)['argocd.token'] ?
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
