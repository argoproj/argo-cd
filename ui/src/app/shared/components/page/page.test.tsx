import * as React from 'react';
import {fireEvent, render, screen} from '@testing-library/react';
import '@testing-library/jest-dom';

import {services} from '../../services';
import {AuthOption} from './page';

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

        expect(window.localStorage.getItem('argocd.sso.autoLoginPaused')).toBe('true');
    });
});
