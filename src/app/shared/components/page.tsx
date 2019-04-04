import { DataLoader, Page as ArgoPage, Toolbar, Utils } from 'argo-ui';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import { Observable } from 'rxjs';

import { AppContext } from '../context';
import { services } from '../services';

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
                    <a key='logout' onClick={() => this.logout()}>Logout</a>,
                ];
                return toolbar;
            })}>
            {(toolbar) => (
                <ArgoPage title={this.props.title} children={this.props.children} toolbar={toolbar} />
            )}
            </DataLoader>
        );
    }

    private async logout() {
        await services.users.logout();
        this.appContext.history.push('/login');
    }

    private get appContext(): AppContext {
        return this.context as AppContext;
    }
}
