import requests from '../../services/requests';
import {getLoginURL} from './page';

jest.mock('../../services', () => ({
    services: {
        users: {get: jest.fn()}
    }
}));

test('login URL respects the configured base href', () => {
    requests.setBaseHRef('/argo-cd');
    try {
        expect(getLoginURL('https://example.com/argo-cd/applications')).toBe('/argo-cd/login?return_url=https%3A%2F%2Fexample.com%2Fargo-cd%2Fapplications');
    } finally {
        requests.setBaseHRef('/');
    }
});
