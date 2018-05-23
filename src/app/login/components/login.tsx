import * as React from 'react';
import { Form, Text } from 'react-form';
import { RouteComponentProps } from 'react-router';

import { FormField } from '../../shared/components';
import { AuthSettings } from '../../shared/models';
import { services } from '../../shared/services';

require('./login.scss');

export interface LoginForm {
    username: string;
    password: string;
}

export class Login extends React.Component<RouteComponentProps<{}>, {authSettings: AuthSettings, loginError: string}> {

    private get returnUrl() {
        return new URLSearchParams(this.props.location.search).get('return_url') || '';
    }

    constructor(props: RouteComponentProps<{}>) {
        super(props);
        this.state = { authSettings: null, loginError: null };
    }

    public async componentDidMount() {
        this.setState({
            authSettings: await services.authService.settings(),
        });
    }

    public render() {
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
                            <a href={`/auth/login?return_url=${encodeURIComponent(this.returnUrl)}`}>
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
                        onSubmit={(params: LoginForm) => this.login(params.username, params.password, this.returnUrl)}
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
                                <div className='argo-form-row__error-msg'>{this.state.loginError}</div>
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

    private async login(username: string, password: string, returnURL: string) {
        try {
            this.setState({loginError: ''});
            await services.userService.login(username, password);
            if (returnURL) {
                const url = new URL(returnURL);
                this.props.history.push(url.pathname + url.search);
            } else {
                this.props.history.push('/applications');
            }
        } catch (e) {
            this.setState({loginError: e.response.body.error});
        }
    }
}
