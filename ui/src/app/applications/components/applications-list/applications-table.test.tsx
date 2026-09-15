import * as AppUtils from '../utils';
import {ContextApis} from '../../../shared/context';

// Minimal app fixture that satisfies getAppUrl / getAppListLink.
const makeApp = (name: string, namespace: string) =>
    ({
        apiVersion: 'argoproj.io/v1alpha1',
        kind: 'Application',
        metadata: {name, namespace},
        spec: {destination: {}},
        status: {sync: {status: 'Synced'}, health: {status: 'Healthy'}, summary: {}}
    }) as any;

const mockCtx = (baseHref: string): ContextApis => {
    const goto = jest.fn();
    return {
        baseHref,
        navigation: {goto},
        popup: {} as any,
        notifications: {} as any
    };
};

describe('getAppListLink baseHref handling (issue #25367)', () => {
    test('href includes baseHref for an app with namespace', () => {
        const ctx = mockCtx('/argocd/');
        const app = makeApp('my-app', 'argocd');
        const link = AppUtils.getAppListLink(ctx, app);

        expect(link.href).toBe('/argocd/applications/argocd/my-app');
        expect(link.path).toBe('/applications/argocd/my-app');
    });

    test('href includes baseHref for root-mounted instance', () => {
        const ctx = mockCtx('/');
        const app = makeApp('my-app', 'argocd');
        const link = AppUtils.getAppListLink(ctx, app);

        expect(link.href).toBe('/applications/argocd/my-app');
        expect(link.path).toBe('/applications/argocd/my-app');
    });

    test('href includes baseHref with custom prefix', () => {
        const ctx = mockCtx('/custom/prefix/');
        const app = makeApp('guestbook', 'default');
        const link = AppUtils.getAppListLink(ctx, app);

        expect(link.href).toBe('/custom/prefix/applications/default/guestbook');
        expect(link.path).toBe('/applications/default/guestbook');
    });
});

describe('Key.ENTER keybinding baseHref behaviour (issue #25367)', () => {
    let windowOpenSpy: jest.SpyInstance;
    const originalEvent = window.event;

    beforeEach(() => {
        windowOpenSpy = jest.spyOn(window, 'open').mockImplementation(() => null);
    });

    afterEach(() => {
        windowOpenSpy.mockRestore();
        Object.defineProperty(window, 'event', {value: originalEvent, writable: true});
    });

    // Simulates the action body extracted from the Key.ENTER keybinding in
    // applications-table.tsx / applications-tiles.tsx.  We test the logic
    // directly rather than mounting the full component tree, because the
    // argo-ui KeybindingProvider + DataLoader + Context wiring is heavyweight
    // and orthogonal to the bug.
    const runEnterAction = (ctx: ContextApis, app: any, kbEvent: Partial<KeyboardEvent> | null) => {
        const appLink = AppUtils.getAppListLink(ctx, app);
        Object.defineProperty(window, 'event', {value: kbEvent, writable: true});

        const ev = window.event as KeyboardEvent;
        if (ev && (ev.ctrlKey || ev.metaKey)) {
            window.open(appLink.href, '_blank');
        } else {
            ctx.navigation.goto(appLink.path);
        }
    };

    test('plain Enter uses SPA navigation (goto)', () => {
        const ctx = mockCtx('/argocd/');
        const app = makeApp('my-app', 'argocd');

        runEnterAction(ctx, app, {ctrlKey: false, metaKey: false});

        expect(ctx.navigation.goto).toHaveBeenCalledWith('/applications/argocd/my-app');
        expect(windowOpenSpy).not.toHaveBeenCalled();
    });

    test('Ctrl+Enter opens new tab with baseHref-prefixed URL', () => {
        const ctx = mockCtx('/argocd/');
        const app = makeApp('my-app', 'argocd');

        runEnterAction(ctx, app, {ctrlKey: true, metaKey: false});

        expect(windowOpenSpy).toHaveBeenCalledWith('/argocd/applications/argocd/my-app', '_blank');
        expect(ctx.navigation.goto).not.toHaveBeenCalled();
    });

    test('Meta+Enter (macOS) opens new tab with baseHref-prefixed URL', () => {
        const ctx = mockCtx('/argocd/');
        const app = makeApp('my-app', 'argocd');

        runEnterAction(ctx, app, {ctrlKey: false, metaKey: true});

        expect(windowOpenSpy).toHaveBeenCalledWith('/argocd/applications/argocd/my-app', '_blank');
        expect(ctx.navigation.goto).not.toHaveBeenCalled();
    });

    test('Ctrl+Enter with custom baseHref produces correct URL', () => {
        const ctx = mockCtx('/custom/prefix/');
        const app = makeApp('guestbook', 'default');

        runEnterAction(ctx, app, {ctrlKey: true, metaKey: false});

        expect(windowOpenSpy).toHaveBeenCalledWith('/custom/prefix/applications/default/guestbook', '_blank');
    });
});
