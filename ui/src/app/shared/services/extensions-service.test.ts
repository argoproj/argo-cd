import {ExtensionsService} from './extensions-service';

describe('ExtensionsService', () => {
    it('matches wildcard resource extensions when resolving resource tabs', () => {
        const service = new ExtensionsService();
        const group = '*.interop26935.example.io';
        const kind = 'Widget26935';
        const title = 'Wildcard extension';

        (window as any).extensionsAPI.registerResourceExtension((() => null) as any, group, kind, title);

        expect(service.getResourceTabs('team.interop26935.example.io', kind)).toEqual(
            expect.arrayContaining([expect.objectContaining({group, kind, title})])
        );
    });
});
