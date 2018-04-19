import { Page as ArgoPage, TopBarProps } from 'argo-ui';
import * as React from 'react';
import { connect } from 'react-redux';
import * as actions from '../actions';

const Component = (props: TopBarProps & { logout: () => any}) => {
    const toolbar = props.toolbar || {};
    props.toolbar.tools = [
        props.toolbar.tools,
        <a key='logout' onClick={() => props.logout()}>Logout</a>,
    ];
    return <ArgoPage {...props} toolbar={toolbar} />;
};

export const Page = connect(null, (dispatch) => ({
    logout: () => dispatch(actions.logout()),
}))(Component);
