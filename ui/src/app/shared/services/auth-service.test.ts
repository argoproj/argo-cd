import {AuthService} from './auth-service';
import requests from './requests';

jest.mock('./requests');

describe('AuthService', () => {
    let service: AuthService;

    beforeEach(() => {
        service = new AuthService();
        jest.clearAllMocks();
    });

    it('caches settings requests', async () => {
        const mockSettings = {execEnabled: true};
        (requests.get as jest.Mock).mockResolvedValue({body: mockSettings});

        const res1 = await service.settings();
        const res2 = await service.settings();

        expect(res1).toEqual(mockSettings);
        expect(res2).toEqual(mockSettings);
        expect(requests.get).toHaveBeenCalledTimes(1);
    });

    it('evicts settings cache on failure', async () => {
        (requests.get as jest.Mock).mockRejectedValueOnce(new Error('Network error'));
        (requests.get as jest.Mock).mockResolvedValueOnce({body: {execEnabled: true}});

        await expect(service.settings()).rejects.toThrow('Network error');

        const res = await service.settings();
        expect(res).toEqual({execEnabled: true});
        expect(requests.get).toHaveBeenCalledTimes(2);
    });

    it('clears settings cache on clearCache call', async () => {
        (requests.get as jest.Mock).mockResolvedValue({body: {execEnabled: true}});

        await service.settings();
        expect(requests.get).toHaveBeenCalledTimes(1);

        service.clearCache();

        await service.settings();
        expect(requests.get).toHaveBeenCalledTimes(2);
    });
});
