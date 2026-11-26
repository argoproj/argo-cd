import * as React from 'react';
import {fireEvent, render, screen} from '@testing-library/react';
import '@testing-library/jest-dom';

import {services} from '../../services';
import requests from '../../services/requests';
import {AuthOption, getLoginURL} from './page';

jest.mock('../../services', () => ({
    services: {
        users: {get: jest.fn()}
    }
}));

describe('AuthOption', () => {
    it('pauses SSO auto-login when the user logs out', async () => {
        window.localStorage.clear();
        jest.mocked(services.users.get).mockResolvedValue({loggedIn: true, username: 'alice', iss: 'https://idp.example.com', groups: []});
        const consoleError = jest.spyOn(console, 'error').mockImplementation(() => undefined);

        try {
            render(<AuthOption />);
            fireEvent.click(await screen.findByText('Log out'));
        } finally {
            consoleError.mockRestore();
        }

        expect(window.localStorage.getItem('argocd.sso.autoLoginPaused:/')).not.toBeNull();
    });
});


test('login URL respects the configured base href', () => {
    requests.setBaseHRef('/argo-cd');
    try {
        expect(getLoginURL('https://example.com/argo-cd/applications')).toBe(
            '/argo-cd/login?return_url=https%3A%2F%2Fexample.com%2Fargo-cd%2Fapplications'
        );
    } finally {
        requests.setBaseHRef('/');
    }
});
