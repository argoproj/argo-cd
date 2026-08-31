// Break the applications-service -> components/utils -> shared/components -> services/index cycle,
// which otherwise leaves ApplicationsService undefined when jest loads this test in isolation.
jest.mock('../../applications/components/utils', () => ({getRootPathByApp: jest.fn(), isApp: jest.fn()}));

import {ApplicationsService} from './applications-service';

describe('getDownloadLogsURL', () => {
    const service = new ApplicationsService();
    const getURL = (viewTimestamps?: boolean) => service.getDownloadLogsURL('my-app', 'argocd', 'my-ns', 'my-pod', {group: '', kind: '', name: ''}, 'main', false, viewTimestamps);

    it('omits timestamps by default', () => {
        const params = new URLSearchParams(getURL().split('?')[1]);
        expect(params.get('download')).toBe('true');
        expect(params.get('timestamps')).toBeNull();
    });

    it('includes timestamps when enabled', () => {
        const params = new URLSearchParams(getURL(true).split('?')[1]);
        expect(params.get('download')).toBe('true');
        expect(params.get('timestamps')).toBe('true');
    });
});
