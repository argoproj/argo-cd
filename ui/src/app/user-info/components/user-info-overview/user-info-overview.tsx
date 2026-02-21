import React, {useCallback, useContext, useRef, useState} from 'react';

import {FormField, NotificationType, SlidingPanel} from 'argo-ui/src/index';
import {Form, FormApi, FormValue, Nested, Text} from 'react-form';
import {DataLoader, ErrorNotification, Page, Spinner} from '../../../shared/components';
import {Context} from '../../../shared/context';
import {services} from '../../../shared/services';

import './user-info-overview.scss';
import {UserInfo} from '../../../shared/models';

// Constants
const CHANGE_PASSWORD_PARAM = 'changePassword';
const ARGOCD_ISSUER = 'argocd';

// Types
interface PasswordFormData {
    currentPassword: string;
    newPassword: string;
    confirmNewPassword: string;
}

// Password form validation
const validatePasswordForm = (params: PasswordFormData) => ({
    currentPassword: !params.currentPassword && 'Current password is required.',
    newPassword: (!params.newPassword && 'New password is required.') || (params.newPassword !== params.confirmNewPassword && 'Passwords do not match.'),
    confirmNewPassword: (!params.confirmNewPassword || params.confirmNewPassword !== params.newPassword) && 'Please confirm your new password.'
});

export const UserInfoComponent = ({userInfo}: {userInfo: UserInfo}) => {
    const appContext = useContext(Context);

    const [isConnecting, setIsConnecting] = useState(false);
    const [showChangePassword, setShowChangePassword] = useState(new URLSearchParams(appContext.history.location.search).get(CHANGE_PASSWORD_PARAM) === 'true');

    const formApiPassword = useRef<FormApi>(null);

    const changePassword = useCallback(
        async (username: string, currentPassword: Nested<FormValue> | FormValue, newPassword: Nested<FormValue> | FormValue) => {
            setIsConnecting(true);
            try {
                await services.accounts.changePassword(username, currentPassword, newPassword);
                appContext.notifications.show({
                    type: NotificationType.Success,
                    content: 'Your password has been successfully updated.'
                });
                setShowChangePassword(false);
                // Clear the URL parameter
                appContext.history.push(appContext.history.location.pathname);
            } catch (e) {
                appContext.notifications.show({
                    content: <ErrorNotification title='Unable to update your password.' e={e} />,
                    type: NotificationType.Error
                });
            } finally {
                setIsConnecting(false);
            }
        },
        [appContext.notifications, appContext.history]
    );

    const clearChangePasswordForm = useCallback(() => {
        formApiPassword.current?.resetAll();
    }, []);

    const updateShowChangePassword = useCallback(
        (val: boolean) => {
            setShowChangePassword(val);
            clearChangePasswordForm();
            const url = `${appContext.history.location.pathname}${val ? `?${CHANGE_PASSWORD_PARAM}=true` : ''}`;
            appContext.history.push(url);
        },
        [appContext.history, clearChangePasswordForm]
    );

    const handleFormSubmit = useCallback(() => {
        formApiPassword.current?.submitForm(null);
    }, []);

    const isPasswordChangeAvailable = userInfo.loggedIn && userInfo.iss === ARGOCD_ISSUER;

    return (
        <Page
            title='User Info'
            toolbar={{
                breadcrumbs: [{title: 'User Info'}],
                actionMenu: isPasswordChangeAvailable
                    ? {
                          items: [
                              {
                                  iconClassName: 'fa fa-lock',
                                  title: 'Update Password',
                                  action: () => updateShowChangePassword(true)
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
                                <>
                                    <p>Username: {userInfo.username}</p>
                                    <p>Issuer: {userInfo.iss}</p>
                                    {userInfo.groups && userInfo.groups.length > 0 && (
                                        <>
                                            <p>Groups:</p>
                                            <ul>
                                                {userInfo.groups.sort().map(group => (
                                                    <li key={group}>{group}</li>
                                                ))}
                                            </ul>
                                        </>
                                    )}
                                </>
                            ) : (
                                <p>You are not logged in</p>
                            )}
                        </div>
                    </div>
                </div>
                {isPasswordChangeAvailable && (
                    <SlidingPanel
                        isShown={showChangePassword}
                        onClose={() => updateShowChangePassword(false)}
                        header={
                            <div>
                                <button className='argo-button argo-button--base' onClick={handleFormSubmit} disabled={isConnecting}>
                                    <Spinner show={isConnecting} style={{marginRight: '5px'}} />
                                    Save New Password
                                </button>{' '}
                                <button onClick={() => updateShowChangePassword(false)} className='argo-button argo-button--base-o' disabled={isConnecting}>
                                    Cancel
                                </button>
                            </div>
                        }>
                        <h4>Update account password</h4>
                        <Form
                            onSubmit={(params: PasswordFormData) => changePassword(userInfo.username, params.currentPassword, params.newPassword)}
                            getApi={api => (formApiPassword.current = api)}
                            validateError={validatePasswordForm}>
                            {formApi => (
                                <form onSubmit={formApi.submitForm} role='form' className='change-password width-control'>
                                    <div className='argo-form-row'>
                                        <FormField
                                            formApi={formApi}
                                            label='Current Password'
                                            field='currentPassword'
                                            component={Text}
                                            componentProps={{
                                                type: 'password',
                                                autoComplete: 'current-password',
                                                disabled: isConnecting
                                            }}
                                        />
                                    </div>
                                    <div className='argo-form-row'>
                                        <FormField
                                            formApi={formApi}
                                            label='New Password'
                                            field='newPassword'
                                            component={Text}
                                            componentProps={{
                                                type: 'password',
                                                autoComplete: 'new-password',
                                                disabled: isConnecting
                                            }}
                                        />
                                    </div>
                                    <div className='argo-form-row'>
                                        <FormField
                                            formApi={formApi}
                                            label='Confirm New Password'
                                            field='confirmNewPassword'
                                            component={Text}
                                            componentProps={{
                                                type: 'password',
                                                autoComplete: 'new-password',
                                                disabled: isConnecting
                                            }}
                                        />
                                    </div>
                                </form>
                            )}
                        </Form>
                    </SlidingPanel>
                )}
            </div>
        </Page>
    );
};

export function UserInfoOverview() {
    return <DataLoader load={() => services.users.get()}>{userInfo => <UserInfoComponent userInfo={userInfo} />}</DataLoader>;
}
