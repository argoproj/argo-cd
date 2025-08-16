import {FormField} from 'argo-ui';
import React, {useContext, useEffect, useState} from 'react';
import {Form, Text} from 'react-form';
import {RouteComponentProps} from 'react-router';
import {AuthSettings} from '../../shared/models';
import {services} from '../../shared/services';
import {Context} from '../../shared/context';

require('./login.scss');

export interface LoginForm {
    username: string;
    password: string;
}

interface State {
    authSettings: AuthSettings | null;
    loginError: string | null;
    loginInProgress: boolean;
}

export function Login(props: RouteComponentProps<{}>) {
    const appContext = useContext(Context);

    const search = new URLSearchParams(props.history.location.search);
    const returnUrl = search.get('return_url') || '';
    const hasSsoLoginError = search.get('has_sso_error') === 'true';

    const [state, setState] = useState<State>({
        authSettings: null,
        loginError: null,
        loginInProgress: false
    });

    useEffect(() => {
        const fetchAuthSettings = async () => {
            const authSettings = await services.authService.settings();
            setState(prevState => ({...prevState, authSettings}));
        };
        fetchAuthSettings();
    }, []);

    const login = async (username: string, password: string, returnURL: string) => {
        try {
            setState(prevState => ({...prevState, loginError: '', loginInProgress: true}));
            appContext.navigation.goto('.', {sso_error: null});
            await services.users.login(username, password);
            setState(prevState => ({...prevState, loginInProgress: false}));
            if (returnURL) {
                const url = new URL(returnURL);
                let redirectURL = url.pathname + url.search;
                // return url already contains baseHref, so we need to remove it
                if (appContext.baseHref != '/' && redirectURL.startsWith(appContext.baseHref)) {
                    redirectURL = redirectURL.substring(appContext.baseHref.length);
                }
                appContext.navigation.goto(redirectURL);
            } else {
                appContext.navigation.goto('/applications');
            }
        } catch (e) {
            setState(prevState => ({...prevState, loginError: e.response.body.error, loginInProgress: false}));
        }
    };

    const {authSettings} = state;
    const ssoConfigured = authSettings && ((authSettings.dexConfig && (authSettings.dexConfig.connectors || []).length > 0) || authSettings.oidcConfig);

    return (
        <div className='login'>
            <div className='login__content show-for-medium'>
                <div className='login__text'>Let's get stuff deployed!</div>
                <div className='argo__logo' />
            </div>
            <div className='login__box'>
                <div className='login__logo width-control'>
                    <img className='logo-image' src='assets/images/argo_o.svg' alt='argo' />
                </div>
                {ssoConfigured && (
                    <div className='login__box_saml width-control'>
                        <a href={`auth/login?return_url=${encodeURIComponent(returnUrl)}`}>
                            <button className='argo-button argo-button--base argo-button--full-width argo-button--xlg'>
                                {(authSettings.oidcConfig && <span>Log in via {authSettings.oidcConfig.name}</span>) ||
                                    (authSettings.dexConfig.connectors.length === 1 && <span>Log in via {authSettings.dexConfig.connectors[0].name}</span>) || (
                                        <span>SSO Login</span>
                                    )}
                            </button>
                        </a>
                        {hasSsoLoginError && <div className='argo-form-row__error-msg'>Login failed.</div>}
                        {authSettings && !authSettings.userLoginsDisabled && (
                            <div className='login__saml-separator'>
                                <span>or</span>
                            </div>
                        )}
                    </div>
                )}
                {authSettings && !authSettings.userLoginsDisabled && (
                    <Form
                        onSubmit={(params: LoginForm) => login(params.username, params.password, returnUrl)}
                        validateError={(params: LoginForm) => ({
                            username: !params.username && 'Username is required',
                            password: !params.password && 'Password is required'
                        })}>
                        {formApi => (
                            <form role='form' className='width-control' onSubmit={formApi.submitForm}>
                                <div className='argo-form-row'>
                                    <FormField
                                        formApi={formApi}
                                        label='Username'
                                        field='username'
                                        component={Text}
                                        componentProps={{name: 'username', autoCapitalize: 'none', autoComplete: 'username'}}
                                    />
                                </div>
                                <div className='argo-form-row'>
                                    <FormField
                                        formApi={formApi}
                                        label='Password'
                                        field='password'
                                        component={Text}
                                        componentProps={{name: 'password', type: 'password', autoComplete: 'password'}}
                                    />
                                    {state.loginError && <div className='argo-form-row__error-msg'>{state.loginError}</div>}
                                </div>
                                <div className='login__form-row'>
                                    <button disabled={state.loginInProgress} className='argo-button argo-button--full-width argo-button--xlg' type='submit'>
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
