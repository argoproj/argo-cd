import * as React from 'react';

test('replays system-level extensions registered before listeners attach', () => {
    try {
        jest.isolateModules(() => {
            const {ExtensionsService} = require('./extensions-service') as typeof import('./extensions-service');
            const component = () => <div />;
            const listener = jest.fn();
            const service = new ExtensionsService();

            (window as any).extensionsAPI.registerSystemLevelExtension(component, 'Test extension', '/test-extension', 'fa-test');
            service.addEventListener('systemLevel', listener);

            expect(listener).toHaveBeenCalledTimes(1);
            expect(listener).toHaveBeenCalledWith({
                component,
                title: 'Test extension',
                path: '/test-extension',
                icon: 'fa-test'
            });

            service.removeEventListener('systemLevel', listener);
        });
    } finally {
        delete (window as any).extensionsAPI;
    }
});
