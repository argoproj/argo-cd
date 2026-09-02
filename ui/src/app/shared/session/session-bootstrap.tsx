import * as React from 'react';

import {UserInfo} from '../models';
import {services} from '../services';
import requests from '../services/requests';
import {httpStatusOf} from '../utils';

export function anonymousUserInfo(): UserInfo {
    return {loggedIn: false, username: '', iss: 'argocd', groups: []};
}

export interface SessionBootstrap {
    userInfo: UserInfo;
    permissionDenied: boolean;
}

export async function fetchUserInfoSafe(): Promise<SessionBootstrap> {
    try {
        return {userInfo: await services.users.get(), permissionDenied: false};
    } catch (err: unknown) {
        switch (httpStatusOf(err)) {
            case 401:
                return {userInfo: anonymousUserInfo(), permissionDenied: false};
            case 403:
                return {userInfo: anonymousUserInfo(), permissionDenied: true};
            default:
                throw err;
        }
    }
}

export const NoPermissionsScreen: React.FC = () => (
    <div style={{display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100vh', flexDirection: 'column', gap: '12px', padding: '0 24px', textAlign: 'center'}}>
        <i className='fa fa-lock' style={{fontSize: '36px', color: '#6d7f8b'}} aria-hidden='true' />
        <h3 style={{margin: 0}}>Your account has no permissions</h3>
        <p style={{color: '#6d7f8b', maxWidth: '480px', margin: 0}}>
            You are signed in, but no Argo CD permissions have been assigned to your account. Contact your administrator to request access.
        </p>
        <button className='argo-button argo-button--base' onClick={() => (window.location.href = requests.toAbsURL('/auth/logout'))}>
            Log out
        </button>
    </div>
);
