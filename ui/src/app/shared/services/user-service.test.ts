import {UserService} from './user-service';
import requests from './requests';
import {services} from './index';

jest.mock('./requests');

describe('UserService', () => {
    let service: UserService;

    beforeEach(() => {
        service = new UserService();
        service.setServices(services.accounts, services.authService);
        jest.clearAllMocks();
    });

    it('clears accounts and authService caches on login', async () => {
        const clearAccountsSpy = jest.spyOn(services.accounts, 'clearCache');
        const clearAuthSpy = jest.spyOn(services.authService, 'clearCache');

        (requests.post as jest.Mock).mockReturnValue({
            send: jest.fn().mockResolvedValue({body: {token: 'test-token'}})
        });

        const res = await service.login('admin', 'password');

        expect(res).toEqual({token: 'test-token'});
        expect(clearAccountsSpy).toHaveBeenCalledTimes(1);
        expect(clearAuthSpy).toHaveBeenCalledTimes(1);
    });

    it('clears accounts and authService caches on logout', async () => {
        const clearAccountsSpy = jest.spyOn(services.accounts, 'clearCache');
        const clearAuthSpy = jest.spyOn(services.authService, 'clearCache');

        (requests.delete as jest.Mock).mockResolvedValue({});

        const res = await service.logout();

        expect(res).toBe(true);
        expect(clearAccountsSpy).toHaveBeenCalledTimes(1);
        expect(clearAuthSpy).toHaveBeenCalledTimes(1);
    });
});
