import * as React from 'react';
import { connect } from 'react-redux';
import { Field, InjectedFormProps, reduxForm } from 'redux-form';

import { AppState } from 'argo-ui';
import * as actions from '../../shared/actions';
import { FormField } from '../../shared/components';
import { AuthSettings } from '../../shared/models';
import { services } from '../../shared/services';
import { State } from '../state';

require('./login.scss');

const required = (value: any) => (value ? undefined : 'Required');

type LoginProperties = InjectedFormProps<LoginForm, {}> & { loginError: string };

export interface LoginForm {
    username: string;
    password: string;
}

export class Form extends React.Component<LoginProperties, {authSettings: AuthSettings}> {

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
                            <a href='/auth/login'>
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
                    <form role='form' className='width-control' onSubmit={props.handleSubmit}>
                        <div className='argo-form-row'>
                            <Field validate={required} name='username' component={FormField} type='text' label='Username'/>
                        </div>
                        <div className='argo-form-row'>
                            <Field validate={required} name='password' component={FormField} type='password' label='Password'/>
                            <div className='argo-form-row__error-msg'>{props.loginError}</div>
                        </div>
                        <div className='login__form-row'>
                            <button className='argo-button argo-button--full-width argo-button--xlg' type='submit'>
                                Sign In
                            </button>
                        </div>
                    </form>
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

const rxLogin = connect((state: AppState<State>) => ({
    loginError: state.page.loginError,
}))(Form);

const Login = reduxForm<LoginForm>({
    form: 'loginForm',
    onSubmit: (values, dispatch) => {
        dispatch(actions.login(values.username, values.password));
    },
})(rxLogin);

export {
    Login,
};
