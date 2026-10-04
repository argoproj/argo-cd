import * as React from 'react';
import '@testing-library/jest-dom';
import {act, fireEvent, render, screen, within} from '@testing-library/react';
import {TopBar} from 'argo-ui/src/components/top-bar/top-bar';
import {createMemoryHistory} from 'history';
import {Route, Router} from 'react-router-dom';
import {NEVER, Subject} from 'rxjs';
import {AuthSettingsCtx, Context} from '../../../shared/context';
import {AbstractApplication, Application, ApplicationWatchEvent, AuthSettings} from '../../../shared/models';
import {services} from '../../../shared/services';
import {ApplicationDetails} from './application-details';
import {getApplicationParent, getApplicationParentPath} from './application-parent';

jest.mock('react', () => ({...jest.requireActual('react'), default: jest.requireActual('react')}));
jest.mock('argo-ui', () => ({...jest.requireActual('argo-ui'), Tooltip: ({children}: {children: React.ReactNode}) => <>{children}</>}));
jest.mock('../../../shared/components/page/page', () => ({
    Page: ({title, toolbar, children}: React.ComponentProps<typeof TopBar> & {children: React.ReactNode}) => (
        <>
            <TopBar title={title} toolbar={toolbar} />
            {children}
        </>
    )
}));
jest.mock('../application-status-panel/application-status-panel', () => ({ApplicationStatusPanel: (): null => null}));
jest.mock('../application-status-panel/appset-status-panel', () => ({ApplicationSetStatusPanel: (): null => null}));
jest.mock('../application-sync-panel/application-sync-panel', () => ({ApplicationSyncPanel: (): null => null}));
jest.mock('../application-deployment-history/application-deployment-history', () => ({ApplicationDeploymentHistory: (): null => null}));
jest.mock('../resource-details/resource-details', () => ({ResourceDetails: (): null => null}));
jest.mock('../resource-details/appset-resource-details', () => ({AppSetResourceDetails: (): null => null}));
jest.mock('./application-details-app-dropdown', () => ({ApplicationsDetailsAppDropdown: (): null => null}));
jest.mock('../../../sidebar/sidebar', () => ({useSidebarTarget: () => ({current: document.body})}));
jest.mock('./application-resource-filter', () => ({
    ...jest.requireActual('./application-resource-filter'),
    Filters: (): null => null
}));
jest.mock('../application-resource-tree/application-resource-tree', () => ({
    ...jest.requireActual('../application-resource-tree/application-resource-tree'),
    ApplicationResourceTree: ({app, nodeMenu}: {app: AbstractApplication; nodeMenu?: (node: unknown) => React.ReactNode}) => {
        const [open, setOpen] = React.useState(false);
        return nodeMenu ? (
            <>
                <button onClick={() => setOpen(value => !value)}>Application menu</button>
                {open && <div data-testid='application-menu'>{nodeMenu({group: 'argoproj.io', kind: app.kind, name: app.metadata.name, namespace: app.metadata.namespace})}</div>}
            </>
        ) : null;
    }
}));

const settings = {trackingMethod: 'annotation', controllerNamespace: 'argocd', appLabelKey: 'app.kubernetes.io/instance', installationID: ''} as AuthSettings;
const source = {repoURL: 'https://example.com/repo.git', path: '.', targetRevision: 'HEAD'};
const makeApp = (kind = 'Application', namespace = 'argocd', parent = 'parent'): Application => ({
    kind,
    metadata: {name: 'child', namespace, annotations: {'argocd.argoproj.io/tracking-id': `${parent}:argoproj.io/${kind}:${namespace}/child`}},
    spec: {
        project: 'default',
        source,
        sources: [],
        destination: {server: 'https://kubernetes.default.svc', namespace: 'default', name: ''}
    },
    status: {sync: {status: 'Synced', revision: 'HEAD', revisions: [], comparedTo: source}, resources: [], observedAt: '', history: [], health: {status: 'Healthy', message: ''}}
});

function renderDetails(application: AbstractApplication, authSettings = settings) {
    const root = application.kind === 'ApplicationSet' ? '/applicationsets' : '/applications';
    const history = createMemoryHistory({initialEntries: [`${root}/${application.metadata.namespace}/${application.metadata.name}`]});
    const goto = jest.fn();
    const context: React.ContextType<typeof Context> = {history, navigation: {goto}, popup: null, notifications: null, baseHref: '/'};
    const page = (currentSettings: AuthSettings) => (
        <Context.Provider value={context}>
            <AuthSettingsCtx.Provider value={currentSettings}>
                <Router history={history}>
                    <Route<{}, '/:resourceType/:appnamespace/:name'>
                        path='/:resourceType/:appnamespace/:name'
                        render={props => <ApplicationDetails {...props} objectListKind={application.kind === 'ApplicationSet' ? 'applicationset' : 'application'} />}
                    />
                </Router>
            </AuthSettingsCtx.Provider>
        </Context.Provider>
    );
    jest.spyOn(services.applications, 'get').mockResolvedValue(application);
    const result = render(page(authSettings));
    return {...result, goto, updateSettings: (value: AuthSettings) => result.rerender(page(value))};
}

describe('Application details Parent action', () => {
    let changes: Subject<ApplicationWatchEvent>;

    beforeEach(() => {
        changes = new Subject();
        services.viewPreferences.init();
        jest.spyOn(services.applications, 'resourceTree').mockResolvedValue({nodes: []});
        jest.spyOn(services.applications, 'watch').mockReturnValue(changes);
        jest.spyOn(services.applications, 'watchResourceTree').mockReturnValue(NEVER);
        jest.spyOn(services.authService, 'settings').mockResolvedValue(settings);
    });

    afterEach(() => {
        expect(services.applications.get).toHaveBeenCalledTimes(1);
        expect(services.authService.settings).not.toHaveBeenCalled();
        jest.restoreAllMocks();
    });

    it.each(['Application', 'ApplicationSet'])('navigates from a tracked %s to its parent Application without fetching it', async kind => {
        const {goto} = renderDetails(makeApp(kind, 'team', 'other-team_parent'));
        const button = await screen.findByRole('button', {name: 'Parent'});
        expect(button).toBeEnabled();
        if (kind === 'Application') {
            expect(button.previousElementSibling).toHaveTextContent('Refresh');
        }
        fireEvent.click(button);
        expect(goto).toHaveBeenCalledWith('/applications/other-team/parent');
    });

    it('includes Parent after Refresh in the Application node menu and navigates to the owning ApplicationSet', async () => {
        const application = makeApp('Application', 'team');
        application.metadata.ownerReferences = [{apiVersion: 'argoproj.io/v1alpha1', kind: 'ApplicationSet', name: 'generator', uid: 'owner-uid', controller: true}];
        const {goto} = renderDetails(application);
        fireEvent.click(await screen.findByRole('button', {name: 'Parent'}));
        expect(goto).toHaveBeenCalledWith('/applicationsets/team/generator');
        goto.mockClear();

        fireEvent.click(screen.getByRole('button', {name: 'Application menu'}));
        const menu = within(screen.getByTestId('application-menu'));
        const parent = await menu.findByText('Parent');
        expect(parent.closest('li').previousElementSibling).toHaveTextContent('Refresh');
        fireEvent.click(parent);
        expect(goto).toHaveBeenCalledWith('/applicationsets/team/generator');
    });

    it('disables Parent in both the toolbar and Application node menu without parent metadata', async () => {
        const application = makeApp();
        application.metadata.annotations = {};
        const {goto} = renderDetails(application);
        const button = await screen.findByRole('button', {name: 'Parent'});
        expect(button).toBeDisabled();
        fireEvent.click(button);

        fireEvent.click(screen.getByRole('button', {name: 'Application menu'}));
        const parent = await within(screen.getByTestId('application-menu')).findByText('Parent');
        expect(parent.closest('li')).toHaveClass('disabled');
        fireEvent.click(parent);
        expect(goto).not.toHaveBeenCalled();
    });

    it('updates both actions when the Application watch reports changed parent metadata', async () => {
        const {goto} = renderDetails(makeApp());
        expect(await screen.findByRole('button', {name: 'Parent'})).toBeEnabled();
        fireEvent.click(screen.getByRole('button', {name: 'Application menu'}));
        await within(screen.getByTestId('application-menu')).findByText('Parent');

        const root = makeApp();
        root.metadata.annotations = {};
        act(() => changes.next({type: 'MODIFIED', application: root}));
        expect(screen.getByRole('button', {name: 'Parent'})).toBeDisabled();
        fireEvent.click(screen.getByRole('button', {name: 'Application menu'}));
        fireEvent.click(screen.getByRole('button', {name: 'Application menu'}));
        expect(within(screen.getByTestId('application-menu')).getByText('Parent').closest('li')).toHaveClass('disabled');

        act(() => changes.next({type: 'MODIFIED', application: makeApp('Application', 'argocd', 'other-team_other-parent')}));
        fireEvent.click(screen.getByRole('button', {name: 'Parent'}));
        expect(goto).toHaveBeenCalledWith('/applications/other-team/other-parent');
        goto.mockClear();
        fireEvent.click(screen.getByRole('button', {name: 'Application menu'}));
        fireEvent.click(screen.getByRole('button', {name: 'Application menu'}));
        fireEvent.click(within(screen.getByTestId('application-menu')).getByText('Parent'));
        expect(goto).toHaveBeenCalledWith('/applications/other-team/other-parent');
    });

    it('rebuilds the menu when tracking settings change', async () => {
        const application = makeApp();
        application.metadata.labels = {'app.kubernetes.io/instance': 'label-parent'};
        const {goto, updateSettings} = renderDetails(application);
        await screen.findByRole('button', {name: 'Parent'});
        updateSettings({...settings, trackingMethod: 'label'});
        fireEvent.click(screen.getByRole('button', {name: 'Parent'}));
        expect(goto).toHaveBeenCalledWith('/applications/argocd/label-parent');
    });
});

describe('getApplicationParent', () => {
    it.each(['Application', 'ApplicationSet'])('resolves the managing Application of a %s', kind => {
        expect(getApplicationParent(makeApp(kind, 'team'), settings)).toEqual({kind: 'Application', name: 'parent', namespace: 'argocd'});
    });

    it('resolves a parent Application outside the controller namespace', () => {
        expect(getApplicationParent(makeApp('Application', 'team', 'other-team_parent'), settings)).toEqual({kind: 'Application', name: 'parent', namespace: 'other-team'});
    });

    it('prefers the controlling ApplicationSet over resource tracking', () => {
        const app = makeApp('Application', 'team');
        app.metadata.ownerReferences = [{apiVersion: 'argoproj.io/v1alpha1', kind: 'ApplicationSet', name: 'generator', uid: 'owner-uid', controller: true}];
        expect(getApplicationParent(app, settings)).toEqual({kind: 'ApplicationSet', name: 'generator', namespace: 'team'});
    });

    it.each([
        {apiVersion: 'other.io/v1', kind: 'ApplicationSet', name: 'wrong-group', controller: true},
        {apiVersion: 'argoproj.io/v1alpha1', kind: 'ApplicationSet', name: 'not-controller', controller: false},
        {kind: 'ApplicationSet', name: 'generator'}
    ])('uses the existing owner helper semantics: %o', owner => {
        const app = makeApp('Application', 'team');
        app.metadata.ownerReferences = [owner];
        expect(getApplicationParent(app, settings)).toEqual({kind: 'ApplicationSet', name: owner.name, namespace: 'team'});
    });

    it('ignores owner references to other resource kinds', () => {
        const app = makeApp();
        app.metadata.ownerReferences = [{apiVersion: 'argoproj.io/v1alpha1', kind: 'Deployment', name: 'wrong-kind', controller: true}];
        expect(getApplicationParent(app, settings)?.kind).toBe('Application');
    });

    it('uses the configured label in label tracking mode', () => {
        const app = makeApp();
        app.metadata.labels = {'custom-tracking': 'team_label-parent'};
        expect(getApplicationParent(app, {...settings, trackingMethod: 'label', appLabelKey: 'custom-tracking'})).toEqual({
            kind: 'Application',
            name: 'label-parent',
            namespace: 'team'
        });
    });

    it('keeps annotations authoritative in annotation+label mode', () => {
        const app = makeApp();
        app.metadata.labels = {'app.kubernetes.io/instance': 'misleading-label'};
        expect(getApplicationParent(app, {...settings, trackingMethod: 'annotation+label'})?.name).toBe('parent');
        app.metadata.annotations = {};
        expect(getApplicationParent(app, {...settings, trackingMethod: 'annotation+label'})).toBeUndefined();
    });

    it.each([
        '',
        'parent',
        'parent:argoproj.io/Application:other/child',
        'parent:argoproj.io/Application:argocd/other',
        'parent:argoproj.io/ApplicationSet:argocd/child',
        'parent:other.io/Application:argocd/child',
        ':argoproj.io/Application:argocd/child',
        'parent:argoproj.io/Application:argocd/child:extra'
    ])('rejects malformed or copied tracking annotations: %s', value => {
        const app = makeApp();
        app.metadata.annotations['argocd.argoproj.io/tracking-id'] = value;
        expect(getApplicationParent(app, settings)).toBeUndefined();
    });

    it('checks the installation ID', () => {
        const app = makeApp();
        expect(getApplicationParent(app, {...settings, installationID: 'local'})).toBeUndefined();
        app.metadata.annotations['argocd.argoproj.io/installation-id'] = 'local';
        expect(getApplicationParent(app, {...settings, installationID: 'local'})?.name).toBe('parent');
    });

    it('does not link an Application to itself', () => {
        expect(getApplicationParent(makeApp('Application', 'argocd', 'child'), settings)).toBeUndefined();
        expect(getApplicationParent(makeApp('Application', 'team', 'team_child'), settings)).toBeUndefined();
    });

    it('does not treat the demo grouping label as tracking', () => {
        const app = makeApp();
        app.metadata.annotations = {};
        app.metadata.labels = {'app.kubernetes.io/part-of': 'gitops-demo'};
        expect(getApplicationParent(app, settings)).toBeUndefined();
        expect(getApplicationParent(app, {...settings, trackingMethod: 'label'})).toBeUndefined();
    });

    it('builds namespace-qualified routes for each parent kind', () => {
        expect(getApplicationParentPath({kind: 'Application', namespace: 'team', name: 'parent'})).toBe('/applications/team/parent');
        expect(getApplicationParentPath({kind: 'ApplicationSet', namespace: 'team', name: 'parent'})).toBe('/applicationsets/team/parent');
    });
});
