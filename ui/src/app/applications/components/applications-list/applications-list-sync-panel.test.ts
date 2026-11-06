import {closeSyncPanelParams, openSyncPanelParams, syncPanelTarget} from './applications-list-sync-panel';

const urlParams = (values: {[key: string]: string}) => {
    const params = new URLSearchParams();
    Object.entries(values).forEach(([key, value]) => value !== null && params.set(key, value));
    return params;
};

describe('Sync panel URL parameters', () => {
    it('does not set the appNamespace list filter when opening the panel', () => {
        expect(openSyncPanelParams('guestbook', 'team-a')).not.toHaveProperty('appNamespace');
    });

    it('reads back the application it was opened for', () => {
        expect(syncPanelTarget(urlParams(openSyncPanelParams('guestbook', 'team-a')))).toEqual({appName: 'guestbook', appNamespace: 'team-a'});
    });

    it('reads the panel namespace, not the list filter, when both are set', () => {
        const params = urlParams({...openSyncPanelParams('guestbook', 'team-a'), appNamespace: 'team-b'});
        expect(syncPanelTarget(params)).toEqual({appName: 'guestbook', appNamespace: 'team-a'});
    });

    it('falls back to appNamespace for links saved before syncAppNamespace existed', () => {
        expect(syncPanelTarget(urlParams({syncApp: 'guestbook', appNamespace: 'team-a'}))).toEqual({appName: 'guestbook', appNamespace: 'team-a'});
    });

    it('reads no application once the panel is closed', () => {
        const params = urlParams({...openSyncPanelParams('guestbook', 'team-a'), ...closeSyncPanelParams});
        expect(syncPanelTarget(params).appName).toBeNull();
    });

    it('clears every parameter it sets when closing the panel', () => {
        const opened = Object.keys(openSyncPanelParams('guestbook', 'team-a'));
        expect(Object.keys(closeSyncPanelParams).sort()).toEqual(opened.sort());
        expect(Object.values(closeSyncPanelParams).every(value => value === null)).toBe(true);
    });
});
