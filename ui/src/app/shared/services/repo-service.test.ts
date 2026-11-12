import requests from './requests';
import {RepositoriesService} from './repo-service';

jest.mock('./requests', () => ({
    __esModule: true,
    default: {get: jest.fn()}
}));

describe('RepositoriesService project-scoped lookups', () => {
    let query: jest.Mock;

    beforeEach(() => {
        query = jest.fn().mockReturnValue(Promise.resolve({body: {items: []}}));
        (requests.get as jest.Mock).mockReset().mockReturnValue({query});
    });

    const repo = 'https://github.com/argoproj/argocd-example-apps.git';
    const encoded = encodeURIComponent(repo);
    const service = new RepositoriesService();

    it.each([
        ['revisions', `/repositories/${encoded}/refs`],
        ['ociTags', `/repositories/${encoded}/oci-tags`],
        ['charts', `/repositories/${encoded}/helmcharts`]
    ] as const)('%s sends the app project', async (method, url) => {
        await service[method](repo, 'project-test');

        expect(requests.get).toHaveBeenCalledWith(url);
        expect(query).toHaveBeenCalledWith({appProject: 'project-test'});
    });

    it.each(['revisions', 'ociTags', 'charts'] as const)('%s sends no appProject when the project is unknown', async method => {
        await service[method](repo);

        expect(query).toHaveBeenCalledWith({});
    });
});
