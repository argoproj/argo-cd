import requests from './requests';
import {CertificatesService} from './cert-service';

jest.mock('./requests', () => ({
    __esModule: true,
    default: {get: jest.fn()}
}));

const mockGet = (body: any) => {
    const query = jest.fn(() => Promise.resolve({body}));
    (requests.get as jest.Mock).mockReturnValue({query});
    return query;
};

describe('CertificatesService.get', () => {
    const service = new CertificatesService();

    beforeEach(() => jest.clearAllMocks());

    it('encodes the server name, so names with a port can be requested', async () => {
        const items = [{serverName: '[ssh.github.com]:443', certType: 'ssh', certSubType: 'ssh-ed25519', certData: '', certInfo: ''}];
        const query = mockGet({items});

        await expect(service.get('[ssh.github.com]:443')).resolves.toEqual(items);
        expect(requests.get).toHaveBeenCalledWith('/certificates/%5Bssh.github.com%5D%3A443/details');
        expect(query).toHaveBeenCalledWith({});
    });

    it('filters by certificate type when given', async () => {
        const query = mockGet({items: []});

        await service.get('cd.example.com', 'https');
        expect(query).toHaveBeenCalledWith({certType: 'https'});
    });

    it('returns an empty list when the response has no items', async () => {
        mockGet({items: null});

        await expect(service.get('cd.example.com')).resolves.toEqual([]);
    });
});
