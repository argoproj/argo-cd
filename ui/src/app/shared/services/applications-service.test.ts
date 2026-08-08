// ui/src/app/shared/services/applications-service.test.ts
import requests from './requests';
import { services } from '.';

jest.mock('./requests', () => ({
    requests: {
        post: jest.fn(),
    },
}));

describe('ApplicationsService - Batch Managed Resources', () => {
    afterEach(() => {
        jest.clearAllMocks();
    });

    test('batchManagedResources posts payload and returns expected response', async () => {
        const mockItems = [{ applicationName: 'app-1', diffs: [] }];
        (requests.post as jest.Mock).mockResolvedValue({ items: mockItems });

        const result = await services.applications.batchManagedResources({
            applicationNames: ['app-1'],
        });

        expect(requests.post).toHaveBeenCalledWith('/api/v1/applications/batch/diffs', {
            applicationNames: ['app-1'],
        });
        expect(result).toEqual(mockItems);
    });
});