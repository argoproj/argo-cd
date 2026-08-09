import * as React from 'react';

import {ExtensionsService} from './extensions-service';

test('replays system-level extensions registered before listeners attach', () => {
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
