/* eslint-env jest */
import {render, screen} from '@testing-library/react';
import * as React from 'react';

import {services} from '../services';
import {fetchUserInfoSafe, NoPermissionsScreen} from './session-bootstrap';

jest.mock('../services', () => ({
    services: {users: {get: jest.fn()}}
}));

const mockGet = services.users.get as unknown as jest.Mock;

describe('fetchUserInfoSafe', () => {
    it('passes a successful response through', async () => {
        const userInfo = {loggedIn: true, username: 'alice', iss: 'https://idp', groups: ['dev']};
        mockGet.mockImplementation(() => Promise.resolve(userInfo));
        await expect(fetchUserInfoSafe()).resolves.toEqual({userInfo, permissionDenied: false});
    });

    it('treats 401 as an anonymous session, not a refusal', async () => {
        mockGet.mockImplementation(() => Promise.reject({status: 401}));
        const session = await fetchUserInfoSafe();
        expect(session.permissionDenied).toBe(false);
        expect(session.userInfo.loggedIn).toBe(false);
    });

    it('reports 403 as a permission refusal', async () => {
        mockGet.mockImplementation(() => Promise.reject({response: {status: 403}}));
        await expect(fetchUserInfoSafe()).resolves.toEqual({
            userInfo: {loggedIn: false, username: '', iss: 'argocd', groups: []},
            permissionDenied: true
        });
    });

    it('rethrows anything else so real failures still surface', async () => {
        mockGet.mockImplementation(() => Promise.reject({status: 500}));
        await expect(fetchUserInfoSafe()).rejects.toEqual({status: 500});
    });

    it('rethrows a failure that carries no status', async () => {
        mockGet.mockImplementation(() => Promise.reject(new Error('network down')));
        await expect(fetchUserInfoSafe()).rejects.toThrow('network down');
    });
});

describe('NoPermissionsScreen', () => {
    it('explains the refusal and offers a way out', () => {
        render(<NoPermissionsScreen />);
        expect(screen.getByText('Your account has no permissions')).toBeInTheDocument();
        expect(screen.getByText(/Contact your administrator/)).toBeInTheDocument();
        // Without a logout control the user is stuck on this screen until they clear the cookie.
        expect(screen.getByRole('button', {name: 'Log out'})).toBeInTheDocument();
    });
});
