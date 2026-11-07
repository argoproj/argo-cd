import {VersionMessage} from '../models';
import requests from './requests';
import {VersionService} from './version-service';

jest.mock('./requests', () => ({
    __esModule: true,
    default: {
        agent: {
            get: jest.fn()
        },
        toAbsURL: jest.fn((url: string) => url)
    }
}));

const mockVersion: VersionMessage = {
    Version: 'v2.14.0',
    BuildDate: '2026-10-14T00:00:00Z',
    GoVersion: 'go1.23.0',
    Compiler: 'gc',
    Platform: 'linux/amd64',
    KustomizeVersion: 'v5.4.3',
    HelmVersion: 'v3.15.0',
    KubectlVersion: 'v1.30.0',
    JsonnetVersion: 'v0.20.0'
};

describe('VersionService', () => {
    let service: VersionService;
    const mockGet = requests.agent.get as jest.Mock;

    beforeEach(() => {
        jest.clearAllMocks();
        service = new VersionService();
    });

    it('fetches version and caches it on first call', async () => {
        mockGet.mockReturnValue(Promise.resolve({body: mockVersion}));

        expect(service.getCachedVersion()).toBeNull();
        const result = await service.version();

        expect(mockGet).toHaveBeenCalledTimes(1);
        expect(requests.toAbsURL).toHaveBeenCalledWith('/api/version');
        expect(result).toEqual(mockVersion);
        expect(service.getCachedVersion()).toEqual(mockVersion);
    });

    it('reuses cached result on subsequent calls without making new HTTP requests', async () => {
        mockGet.mockReturnValue(Promise.resolve({body: mockVersion}));

        const result1 = await service.version();
        const result2 = await service.version();

        expect(mockGet).toHaveBeenCalledTimes(1);
        expect(result1).toEqual(mockVersion);
        expect(result2).toEqual(mockVersion);
    });

    it('shares the in-flight promise for concurrent calls', async () => {
        let resolveRequest: (val: any) => void;
        const pendingPromise = new Promise(resolve => {
            resolveRequest = resolve;
        });
        mockGet.mockReturnValue(pendingPromise);

        const call1 = service.version();
        const call2 = service.version();

        expect(mockGet).toHaveBeenCalledTimes(1);

        resolveRequest!({body: mockVersion});
        const [result1, result2] = await Promise.all([call1, call2]);

        expect(result1).toEqual(mockVersion);
        expect(result2).toEqual(mockVersion);
    });

    it('clears cached promise on failure so subsequent calls can retry', async () => {
        mockGet.mockReturnValueOnce(Promise.reject(new Error('Network error')));
        mockGet.mockReturnValueOnce(Promise.resolve({body: mockVersion}));

        await expect(service.version()).rejects.toThrow('Network error');
        expect(service.getCachedVersion()).toBeNull();

        // Retry should trigger a new HTTP request and succeed
        const retryResult = await service.version();
        expect(mockGet).toHaveBeenCalledTimes(2);
        expect(retryResult).toEqual(mockVersion);
        expect(service.getCachedVersion()).toEqual(mockVersion);
    });
});
