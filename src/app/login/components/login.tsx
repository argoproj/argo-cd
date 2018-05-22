import * as React from 'react';
import { Form, Text } from 'react-form';
import { connect } from 'react-redux';

import { AppState } from 'argo-ui';
import * as actions from '../../shared/actions';
import {FormField } from '../../shared/components';
import { AuthSettings } from '../../shared/models';
import { services } from '../../shared/services';
import { State } from '../state';

require('./login.scss');

interface LoginProperties {
    returnUrl: string;
    loginError: string;
    login: (username: string, password: string, returnURL: string ) => any;
}

export interface LoginForm {
    username: string;
    password: string;
}

export class LoginForm extends React.Component<LoginProperties, {authSettings: AuthSettings}> {

    constructor(props: LoginProperties) {
        super(props);
        this.state = { authSettings: null };
    }

    public async componentDidMount() {
        this.setState({
            authSettings: await services.authService.settings(),
        });
    }

    public render() {
        const props = this.props;
        const authSettings = this.state.authSettings;
        return (
            <div className='login'>
                <div className='login__content'>
                    <div className='login__text'>
                        Let's get stuff deployed!
                    </div>
                    <div className='argo__logo'/>
                </div>
                <div className='login__box'>
                    <div className='login__logo width-control'>
                        <img className='logo-image' src='assets/images/argo_o.svg' alt='argo'/>
                    </div>
                    {authSettings && authSettings.dexConfig && (authSettings.dexConfig.connectors || []).length > 0 && (
                        <div className='login__box_saml width-control'>
                            <a href={`/auth/login?return_url=${encodeURIComponent(this.props.returnUrl)}`}>
                                <button className='argo-button argo-button--base argo-button--full-width argo-button--xlg'>
                                    {authSettings.dexConfig.connectors.length === 1 && (
                                        <span>Login via {authSettings.dexConfig.connectors[0].name}</span>
                                    ) || (
                                        <span>SSO Login</span>
                                    )}
                                </button>
                            </a>
                            <div className='login__saml-separator'><span>or</span></div>
                        </div>
                    )}
                    <Form
                        onSubmit={(params: LoginForm) => this.props.login(params.username, params.password, this.props.returnUrl)}
                        validateError={(params: LoginForm) => ({
                            username: !params.username && 'Username is required',
                            password: !params.password && 'Password is required',
                        })}>
                        {(formApi) => (
                            <form role='form' className='width-control' onSubmit={formApi.submitForm}>
                            <div className='argo-form-row'>
                                <FormField formApi={formApi} label='Username' field='username' component={Text}/>
                            </div>
                            <div className='argo-form-row'>
                                <FormField formApi={formApi} label='Password' field='password' component={Text} componentProps={{type: 'password'}}/>
                                <div className='argo-form-row__error-msg'>{props.loginError}</div>
                            </div>
                            <div className='login__form-row'>
                                <button className='argo-button argo-button--full-width argo-button--xlg' type='submit'>
                                    Sign In
                                </button>
                            </div>
                        </form>
                        )}
                    </Form>
                    <div className='login__footer'>
                        <a href='https://argoproj.io' target='_blank'>
                            <img className='logo-image' src='assets/images/argologo.svg' alt='argo'/>
                        </a>
                    </div>
                </div>
            </div>
        );
    }
}

const Login = connect((state: AppState<State>) => ({
    returnUrl: new URLSearchParams(state.router.location.search).get('return_url') || '',
    loginError: state.page.loginError,
}), (dispatch) => ({
    login: (username: string, password: string, returnURL: string) => dispatch(actions.login(username, password, returnURL)),
}))(LoginForm);

export {
    Login,
};
