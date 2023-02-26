import * as React from 'react';

import {FormField, NotificationType, SlidingPanel} from 'argo-ui/src/index';
import * as PropTypes from 'prop-types';
import {Form, FormApi, FormValue, Nested, Text} from 'react-form';
import {RouteComponentProps} from 'react-router';
import {DataLoader, ErrorNotification, Page, Spinner} from '../../../shared/components';
import {AppContext} from '../../../shared/context';
import {services} from '../../../shared/services';

import './user-info-overview.scss';
import {withTranslation} from 'react-i18next';
import en from '../../../locales/en';

class UserInfoOverviewComponent extends React.Component<RouteComponentProps<any>, {connecting: boolean}> {
    public static contextTypes = {
        router: PropTypes.object,
        apis: PropTypes.object,
        history: PropTypes.object
    };

    private formApiPassword: FormApi;

    constructor(props: RouteComponentProps<any>) {
        super(props);
        this.state = {connecting: false};
    }

    public render() {
        return (
            <DataLoader key='userInfo' load={() => services.users.get()}>
                {userInfo => (
                    <Page
                        title={this.props.t('user-info-overview.breadcrumbs.0', en['user-info-overview.breadcrumbs.0'])}
                        toolbar={{
                            breadcrumbs: [{title: this.props.t('user-info-overview.breadcrumbs.0', en['user-info-overview.breadcrumbs.0'])}],
                            actionMenu:
                                userInfo.loggedIn && userInfo.iss === 'argocd'
                                    ? {
                                          items: [
                                              {
                                                  iconClassName: 'fa fa-lock',
                                                  title: this.props.t('user-info-overview.toolbar.update-password', en['user-info-overview.toolbar.update-password']),
                                                  action: () => (this.showChangePassword = true)
                                              }
                                          ]
                                      }
                                    : {items: []}
                        }}>
                        <div>
                            <div className='user-info'>
                                <div className='argo-container'>
                                    <div className='user-info-overview__panel white-box'>
                                        {userInfo.loggedIn ? (
                                            <React.Fragment key='userInfoInner'>
                                                <p key='username'>
                                                    {this.props.t('user-info-overview.user-info.username', en['user-info-overview.user-info.username'], {
                                                        username: userInfo.username
                                                    })}
                                                </p>
                                                <p key='iss'>
                                                    {this.props.t('user-info-overview.user-info.issuer', en['user-info-overview.user-info.issuer'], {issuer: userInfo.iss})}
                                                </p>
                                                {userInfo.groups && (
                                                    <React.Fragment key='userInfo4'>
                                                        <p>{this.props.t('user-info-overview.user-info.groups', en['user-info-overview.user-info.groups'])}</p>
                                                        <ul>
                                                            {userInfo.groups.map(group => (
                                                                <li key={group}>{group}</li>
                                                            ))}
                                                        </ul>
                                                    </React.Fragment>
                                                )}
                                            </React.Fragment>
                                        ) : (
                                            <p key='loggedOutMessage'>{this.props.t('user-info-overview.user-info.logged-out', en['user-info-overview.user-info.logged-out'])}</p>
                                        )}
                                    </div>
                                </div>
                            </div>
                            {userInfo.loggedIn && userInfo.iss === 'argocd' ? (
                                <SlidingPanel
                                    isShown={this.showChangePassword}
                                    onClose={() => (this.showChangePassword = false)}
                                    header={
                                        <div>
                                            <button
                                                className='argo-button argo-button--base'
                                                onClick={() => {
                                                    this.formApiPassword.submitForm(null);
                                                }}>
                                                <Spinner show={this.state.connecting} style={{marginRight: '5px'}} />
                                                {this.props.t('user-info-overview.sliding-panel.save-new-password', en['user-info-overview.sliding-panel.save-new-password'])}
                                            </button>{' '}
                                            <button onClick={() => (this.showChangePassword = false)} className='argo-button argo-button--base-o'>
                                                {this.props.t('cancel', en.cancel)}
                                            </button>
                                        </div>
                                    }>
                                    <h4>{this.props.t('user-info-overview.sliding-panel.title', en['user-info-overview.sliding-panel.title'])}</h4>
                                    <Form
                                        onSubmit={params => this.changePassword(userInfo.username, params.currentPassword, params.newPassword)}
                                        getApi={api => (this.formApiPassword = api)}
                                        defaultValues={{type: 'git'}}
                                        validateError={(params: {currentPassword: string; newPassword: string; confirmNewPassword: string}) => ({
                                            currentPassword:
                                                !params.currentPassword &&
                                                this.props.t(
                                                    'user-info-overview.sliding-panel.current-password-required',
                                                    en['user-info-overview.sliding-panel.current-password-required']
                                                ),
                                            newPassword:
                                                (!params.newPassword &&
                                                    this.props.t(
                                                        'user-info-overview.sliding-panel.new-password-required',
                                                        en['user-info-overview.sliding-panel.new-password-required']
                                                    )) ||
                                                (params.newPassword !== params.confirmNewPassword &&
                                                    this.props.t(
                                                        'user-info-overview.sliding-panel.confirm-new-password',
                                                        en['user-info-overview.sliding-panel.confirm-new-password']
                                                    )),
                                            confirmNewPassword:
                                                (!params.confirmNewPassword || params.confirmNewPassword !== params.newPassword) &&
                                                this.props.t('user-info-overview.sliding-panel.confirm-new-password', en['user-info-overview.sliding-panel.confirm-new-password'])
                                        })}>
                                        {formApi => (
                                            <form onSubmit={formApi.submitForm} role='form' className='change-password width-control'>
                                                <div className='argo-form-row'>
                                                    <FormField
                                                        formApi={formApi}
                                                        label={this.props.t(
                                                            'user-info-overview.sliding-panel.current-password.label',
                                                            en['user-info-overview.sliding-panel.current-password.label']
                                                        )}
                                                        field='currentPassword'
                                                        component={Text}
                                                        componentProps={{type: 'password'}}
                                                    />
                                                </div>
                                                <div className='argo-form-row'>
                                                    <FormField
                                                        formApi={formApi}
                                                        label={this.props.t(
                                                            'user-info-overview.sliding-panel.new-password.label',
                                                            en['user-info-overview.sliding-panel.new-password.label']
                                                        )}
                                                        field='newPassword'
                                                        component={Text}
                                                        componentProps={{type: 'password'}}
                                                    />
                                                </div>
                                                <div className='argo-form-row'>
                                                    <FormField
                                                        formApi={formApi}
                                                        label={this.props.t(
                                                            'user-info-overview.sliding-panel.confirm-new-password.label',
                                                            en['user-info-overview.sliding-panel.confirm-new-password.label']
                                                        )}
                                                        field='confirmNewPassword'
                                                        component={Text}
                                                        componentProps={{type: 'password'}}
                                                    />
                                                </div>
                                            </form>
                                        )}
                                    </Form>
                                </SlidingPanel>
                            ) : (
                                <div />
                            )}
                        </div>
                    </Page>
                )}
            </DataLoader>
        );
    }

    private async changePassword(username: string, currentPassword: Nested<FormValue> | FormValue, newPassword: Nested<FormValue> | FormValue) {
        try {
            await services.accounts.changePassword(username, currentPassword, newPassword);
            this.appContext.apis.notifications.show({
                type: NotificationType.Success,
                content: this.props.t('user-info-overview.change-password.success', en['user-info-overview.change-password.success'])
            });
            this.showChangePassword = false;
        } catch (e) {
            this.appContext.apis.notifications.show({
                content: <ErrorNotification title={this.props.t('user-info-overview.change-password.failed', en['user-info-overview.change-password.failed'])} e={e} />,
                type: NotificationType.Error
            });
        }
    }

    // Whether to show the HTTPS repository connection dialogue on the page
    private get showChangePassword() {
        return new URLSearchParams(this.props.location.search).get('changePassword') === 'true';
    }

    private set showChangePassword(val: boolean) {
        this.clearChangePasswordForm();
        this.appContext.router.history.push(`${this.props.match.url}?changePassword=${val}`);
    }

    // Empty all fields in HTTPS repository form
    private clearChangePasswordForm() {
        this.formApiPassword.resetAll();
    }

    private get appContext(): AppContext {
        return this.context as AppContext;
    }
}

export const UserInfoOverview = withTranslation()(UserInfoOverviewComponent);
