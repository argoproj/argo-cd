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

export function Login(props: RouteComponentProps<{}>) {
    const appContext = useContext(Context);

    const search = new URLSearchParams(props.history.location.search);
    const returnUrl = search.get('return_url') || '';
    const hasSsoLoginError = search.get('has_sso_error') === 'true';

    const [authSettings, setAuthSettings] = useState<AuthSettings | null>(null);
    const [loginError, setLoginError] = useState<string | null>(null);
    const [loginInProgress, setLoginInProgress] = useState<boolean>(false);

    useEffect(() => {
        (async () => {
            const authSettings = await services.authService.settings();
            setAuthSettings(authSettings);
        })();
    }, []);

    const login = async (username: string, password: string, returnURL: string) => {
        try {
            setLoginError('');
            setLoginInProgress(true);
            appContext.navigation.goto('.', {sso_error: null});

            await services.users.login(username, password);

            // Verify session is established before navigating
            const userInfo = await services.users.get();
            if (!userInfo.loggedIn) {
                throw new Error('Session not established after login');
            }

            // Use window.location.replace to force full page reload and ensure cookie is available
            // This also clears any cached DataLoader results
            const basehref = appContext.baseHref === '/' ? '' : appContext.baseHref;
            let redirectPath = '/applications';

            if (returnURL) {
                try {
                    // Handle both absolute and relative URLs
                    let url: URL;
                    if (returnURL.startsWith('http://') || returnURL.startsWith('https://')) {
                        url = new URL(returnURL);
                    } else {
                        // Relative URL - use current origin
                        url = new URL(returnURL, window.location.origin);
                    }
                    redirectPath = url.pathname + url.search;
                    // return url already contains baseHref, so we need to remove it
                    if (appContext.baseHref != '/' && redirectPath.startsWith(appContext.baseHref)) {
                        redirectPath = redirectPath.substring(appContext.baseHref.length);
                    }
                } catch (e) {
                    // If URL parsing fails, use default
                    console.error('Failed to parse return URL:', e, 'returnURL:', returnURL);
                }
            }

            const finalUrl = basehref + redirectPath;

            // Navigate immediately - don't set loginInProgress to false as page will reload
            // Use replace to avoid adding to history
            window.location.replace(finalUrl);
        } catch (e: any) {
            const errorMessage = e?.response?.body?.error || e?.response?.data?.error || e?.message || 'Login failed';
            setLoginError(errorMessage);
            setLoginInProgress(false);
        }
    };

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
                        onSubmitFailure={() => {}}
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
                                    {loginError && <div className='argo-form-row__error-msg'>{loginError}</div>}
                                </div>
                                <div className='login__form-row'>
                                    <button disabled={loginInProgress} className='argo-button argo-button--full-width argo-button--xlg' type='submit'>
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
