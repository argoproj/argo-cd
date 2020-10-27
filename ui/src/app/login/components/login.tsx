import {FormField} from 'argo-ui';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import {Form, Text} from 'react-form';
import {RouteComponentProps} from 'react-router';

import {AppContext} from '../../shared/context';
import {AuthSettings} from '../../shared/models';
import {services} from '../../shared/services';

require('./login.scss');

export interface LoginForm {
    username: string;
    password: string;
}

interface State {
    authSettings: AuthSettings;
    loginError: string;
    loginInProgress: boolean;
    returnUrl: string;
    ssoLoginError: string;
}

export class Login extends React.Component<RouteComponentProps<{}>, State> {
    public static contextTypes = {
        apis: PropTypes.object
    };

    public static getDerivedStateFromProps(props: RouteComponentProps<{}>): Partial<State> {
        const search = new URLSearchParams(props.history.location.search);
        const returnUrl = search.get('return_url') || '';
        const ssoLoginError = search.get('sso_error') || '';
        return {ssoLoginError, returnUrl};
    }

    constructor(props: RouteComponentProps<{}>) {
        super(props);
        this.state = {authSettings: null, loginError: null, returnUrl: null, ssoLoginError: null, loginInProgress: false};
    }

    public async componentDidMount() {
        this.setState({
            authSettings: await services.authService.settings()
        });
    }

    public render() {
        const authSettings = this.state.authSettings;
        const ssoConfigured = authSettings && ((authSettings.dexConfig && (authSettings.dexConfig.connectors || []).length > 0) || authSettings.oidcConfig);
        return (
            <div className='login'>
                <div className='login__content'>
                    <div className='login__text'>Let's get stuff deployed!</div>
                    <div className='argo__logo' />
                </div>
                <div className='login__box'>
                    <div className='login__logo width-control'>
                        <img className='logo-image' src='assets/images/argo_o.svg' alt='argo' />
                    </div>
                    {ssoConfigured && (
                        <div className='login__box_saml width-control'>
                            <a href={`auth/login?return_url=${encodeURIComponent(this.state.returnUrl)}`}>
                                <button className='argo-button argo-button--base argo-button--full-width argo-button--xlg'>
                                    {(authSettings.oidcConfig && <span>Log in via {authSettings.oidcConfig.name}</span>) ||
                                        (authSettings.dexConfig.connectors.length === 1 && <span>Log in via {authSettings.dexConfig.connectors[0].name}</span>) || (
                                            <span>SSO Login</span>
                                        )}
                                </button>
                            </a>
                            {this.state.ssoLoginError && <div className='argo-form-row__error-msg'>{this.state.ssoLoginError}</div>}
                            {authSettings && !authSettings.userLoginsDisabled && (
                                <div className='login__saml-separator'>
                                    <span>or</span>
                                </div>
                            )}
                        </div>
                    )}
                    {authSettings && !authSettings.userLoginsDisabled && (
                        <Form
                            onSubmit={(params: LoginForm) => this.login(params.username, params.password, this.state.returnUrl)}
                            validateError={(params: LoginForm) => ({
                                username: !params.username && 'Username is required',
                                password: !params.password && 'Password is required'
                            })}>
                            {formApi => (
                                <form role='form' className='width-control' onSubmit={formApi.submitForm}>
                                    <div className='argo-form-row'>
                                        <FormField formApi={formApi} label='Username' field='username' component={Text} />
                                    </div>
                                    <div className='argo-form-row'>
                                        <FormField formApi={formApi} label='Password' field='password' component={Text} componentProps={{type: 'password'}} />
                                        {this.state.loginError && <div className='argo-form-row__error-msg'>{this.state.loginError}</div>}
                                    </div>
                                    <div className='login__form-row'>
                                        <button disabled={this.state.loginInProgress} className='argo-button argo-button--full-width argo-button--xlg' type='submit'>
                                            Sign In
                                        </button>
                                    </div>
                                </form>
                            )}
                        </Form>
                    )}
                    {authSettings && authSettings.userLoginsDisabled && !ssoConfigured && (
                        <div className='argo-form-row__error-msg'>Login is disabled. Please contact your system administrator.</div>
                    )}
                    <div className='login__footer'>
                        <a href='https://argoproj.io' target='_blank'>
                            <img className='logo-image' src='assets/images/argologo.svg' alt='argo' />
                        </a>
                    </div>
                </div>
            </div>
        );
    }

    private async login(username: string, password: string, returnURL: string) {
        try {
            this.setState({loginError: '', loginInProgress: true});
            this.appContext.apis.navigation.goto('.', {sso_error: null});
            await services.users.login(username, password);
            this.setState({loginInProgress: false});
            if (returnURL) {
                const url = new URL(returnURL);
                this.appContext.apis.navigation.goto(url.pathname + url.search);
            } else {
                this.appContext.apis.navigation.goto('/applications');
            }
        } catch (e) {
            this.setState({loginError: e.response.body.error, loginInProgress: false});
        }
    }

    private get appContext(): AppContext {
        return this.context as AppContext;
    }
}
