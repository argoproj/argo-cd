import { Page as ArgoPage, TopBarProps } from 'argo-ui';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import { AppContext } from '../context';
import { services } from '../services';

export class Page extends React.Component<TopBarProps> {
    public static contextTypes = {
        router: PropTypes.object,
        history: PropTypes.object,
    };

    public render() {
        const toolbar = this.props.toolbar || {};
        toolbar.tools = [
            toolbar.tools,
            <a  style={{float: 'right', paddingLeft: '1em'}} key='logout' onClick={() => this.logout()}>Logout</a>,
        ];
        return <ArgoPage {...this.props} toolbar={toolbar} />;
    }

    private async logout() {
        await services.userService.logout();
        this.appContext.history.push('/login');
    }

    private get appContext(): AppContext {
        return this.context as AppContext;
    }
}
