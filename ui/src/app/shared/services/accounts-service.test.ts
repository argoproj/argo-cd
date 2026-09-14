import {AccountsService} from './accounts-service';
import requests from './requests';

jest.mock('./requests');

describe('AccountsService', () => {
    let service: AccountsService;

    beforeEach(() => {
        service = new AccountsService();
        jest.clearAllMocks();
    });

    it('caches canI requests with identical parameters', async () => {
        (requests.get as jest.Mock).mockResolvedValue({body: {value: 'yes'}});

        const res1 = await service.canI('logs', 'get', 'app1');
        const res2 = await service.canI('logs', 'get', 'app1');

        expect(res1).toBe(true);
        expect(res2).toBe(true);
        expect(requests.get).toHaveBeenCalledTimes(1);
    });

    it('issues separate requests for different parameters', async () => {
        (requests.get as jest.Mock).mockResolvedValue({body: {value: 'yes'}});

        await service.canI('logs', 'get', 'app1');
        await service.canI('exec', 'create', 'app1');

        expect(requests.get).toHaveBeenCalledTimes(2);
    });

    it('evicts cache on failure to allow retry', async () => {
        (requests.get as jest.Mock).mockRejectedValueOnce(new Error('Network error'));
        (requests.get as jest.Mock).mockResolvedValueOnce({body: {value: 'yes'}});

        await expect(service.canI('logs', 'get', 'app1')).rejects.toThrow('Network error');

        const res = await service.canI('logs', 'get', 'app1');
        expect(res).toBe(true);
        expect(requests.get).toHaveBeenCalledTimes(2);
    });

    it('clears cache on clearCache call', async () => {
        (requests.get as jest.Mock).mockResolvedValue({body: {value: 'yes'}});

        await service.canI('logs', 'get', 'app1');
        expect(requests.get).toHaveBeenCalledTimes(1);

        service.clearCache();

        await service.canI('logs', 'get', 'app1');
        expect(requests.get).toHaveBeenCalledTimes(2);
    });
});
