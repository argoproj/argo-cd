import * as React from 'react';

import {FormField, NotificationType, SlidingPanel} from 'argo-ui/src/index';
import * as PropTypes from 'prop-types';
import {Form, FormApi, FormValue, Nested, Text} from 'react-form';
import {RouteComponentProps} from 'react-router';
import {DataLoader, ErrorNotification, Page, Spinner} from '../../../shared/components';
import {AppContext} from '../../../shared/context';
import {services} from '../../../shared/services';

import './user-info-overview.scss';

export class UserInfoOverview extends React.Component<RouteComponentProps<any>, {connecting: boolean}> {
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
                        title='User Info'
                        toolbar={{
                            breadcrumbs: [{title: 'User Info'}],
                            actionMenu:
                                userInfo.loggedIn && userInfo.iss === 'argocd'
                                    ? {
                                          items: [
                                              {
                                                  iconClassName: 'fa fa-lock',
                                                  title: 'Update Password',
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
                                                <p key='username'>Username: {userInfo.username}</p>
                                                <p key='iss'>Issuer: {userInfo.iss}</p>
                                                {userInfo.groups && (
                                                    <React.Fragment key='userInfo4'>
                                                        <p>Groups:</p>
                                                        <ul>
                                                            {userInfo.groups.map(group => (
                                                                <li key={group}>{group}</li>
                                                            ))}
                                                        </ul>
                                                    </React.Fragment>
                                                )}
                                            </React.Fragment>
                                        ) : (
                                            <p key='loggedOutMessage'>You are not logged in</p>
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
                                                Save New Password
                                            </button>{' '}
                                            <button onClick={() => (this.showChangePassword = false)} className='argo-button argo-button--base-o'>
                                                Cancel
                                            </button>
                                        </div>
                                    }>
                                    <h4>Update account password</h4>
                                    <Form
                                        onSubmit={params => this.changePassword(userInfo.username, params.currentPassword, params.newPassword)}
                                        getApi={api => (this.formApiPassword = api)}
                                        defaultValues={{type: 'git'}}
                                        validateError={(params: {currentPassword: string; newPassword: string; confirmNewPassword: string}) => ({
                                            currentPassword: !params.currentPassword && 'Current password is required.',
                                            newPassword:
                                                (!params.newPassword && 'New password is required.') ||
                                                (params.newPassword !== params.confirmNewPassword && 'Confirm your new password.'),
                                            confirmNewPassword: (!params.confirmNewPassword || params.confirmNewPassword !== params.newPassword) && 'Confirm your new password.'
                                        })}>
                                        {formApi => (
                                            <form onSubmit={formApi.submitForm} role='form' className='change-password width-control'>
                                                <div className='argo-form-row'>
                                                    <FormField
                                                        formApi={formApi}
                                                        label='Current Password'
                                                        field='currentPassword'
                                                        component={Text}
                                                        componentProps={{type: 'password'}}
                                                    />
                                                </div>
                                                <div className='argo-form-row'>
                                                    <FormField formApi={formApi} label='New Password' field='newPassword' component={Text} componentProps={{type: 'password'}} />
                                                </div>
                                                <div className='argo-form-row'>
                                                    <FormField
                                                        formApi={formApi}
                                                        label='Confirm New Password'
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
            this.appContext.apis.notifications.show({type: NotificationType.Success, content: 'Your password has been successfully updated.'});
            this.showChangePassword = false;
        } catch (e) {
            this.appContext.apis.notifications.show({
                content: <ErrorNotification title='Unable to update your password.' e={e} />,
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
