import * as React from 'react';
import {fireEvent, render, screen} from '@testing-library/react';
import {Form, FormApi} from 'argo-ui';
import * as models from '../../models';
import {ApplicationSelector, getSelectedApps} from './application-selector';

jest.mock('react-virtualized/dist/commonjs/AutoSizer', () => ({
    __esModule: true,
    default: ({children}: {children: (size: {width: number; height: number}) => React.ReactNode}) => children({width: 800, height: 600})
}));

jest.mock('react-virtualized/dist/commonjs/List', () => ({
    __esModule: true,
    default: ({rowCount, rowRenderer}: {rowCount: number; rowRenderer: (props: {index: number; key: string; style: React.CSSProperties}) => React.ReactNode}) => (
        <div data-testid='virtualized-list' data-count={rowCount}>
            {[0, 1, rowCount - 1].map(index => rowRenderer({index, key: String(index), style: {}}))}
        </div>
    )
}));

const newApp = (name: string, namespace = 'argocd', syncStatus: models.SyncStatusCode = 'OutOfSync') =>
    ({
        metadata: {name, namespace},
        spec: {project: 'default'},
        status: {sync: {status: syncStatus}, health: {status: 'Healthy'}}
    }) as unknown as models.Application;

const appA = newApp('app-a', 'argocd', 'Synced');
const appB = newApp('app-b');
const appC = newApp('app-c');

const renderSelector = (apps: models.Application[]) => {
    let api: FormApi = null;
    const ui = (list: models.Application[]) => (
        <Form defaultValues={{selectedApps: []}} getApi={formApi => (api = formApi)}>
            {formApi => <ApplicationSelector apps={list} formApi={formApi} />}
        </Form>
    );
    const result = render(ui(apps));
    return {getApi: () => api, rerender: (list: models.Application[]) => result.rerender(ui(list))};
};

const checkboxOf = (name: string) => screen.getByText(name, {exact: false}).closest('label').querySelector('input') as HTMLInputElement;

describe('getSelectedApps', () => {
    it('returns the selected apps by identity', () => {
        expect(getSelectedApps([appA, appB, appC], {selectedApps: ['argocd_app-c', 'argocd_app-a']})).toEqual([appA, appC]);
    });

    it('returns nothing when no apps are selected', () => {
        expect(getSelectedApps([appA, appB], {})).toEqual([]);
    });

    it('ignores selected apps that are no longer in the list', () => {
        expect(getSelectedApps([appA], {selectedApps: ['argocd_app-a', 'argocd_app-b']})).toEqual([appA]);
    });

    it('distinguishes apps with the same name in different namespaces', () => {
        const other = newApp('app-a', 'team-a');
        expect(getSelectedApps([appA, other], {selectedApps: ['team-a_app-a']})).toEqual([other]);
    });

    it('supports app names containing dots', () => {
        const dotted = newApp('my.app');
        expect(getSelectedApps([appA, dotted], {selectedApps: ['argocd_my.app']})).toEqual([dotted]);
    });
});

describe('ApplicationSelector', () => {
    it('keeps the selection on the same app when a new app is prepended to the list', () => {
        const {getApi, rerender} = renderSelector([appA, appB]);
        fireEvent.click(checkboxOf('app-b'));
        expect(checkboxOf('app-b').checked).toBe(true);

        // The applications list prepends apps that are created while the panel is open.
        rerender([appC, appA, appB]);

        expect(checkboxOf('app-b').checked).toBe(true);
        expect(checkboxOf('app-a').checked).toBe(false);
        expect(checkboxOf('app-c').checked).toBe(false);
        expect(getSelectedApps([appC, appA, appB], getApi().getFormState().values)).toEqual([appB]);
    });

    it('unselects an app when its checkbox is clicked again', () => {
        const {getApi} = renderSelector([appA, appB]);
        fireEvent.click(checkboxOf('app-a'));
        fireEvent.click(checkboxOf('app-b'));
        fireEvent.click(checkboxOf('app-a'));
        expect(getSelectedApps([appA, appB], getApi().getFormState().values)).toEqual([appB]);
    });

    it('selects all, out of sync and no apps', () => {
        const {getApi} = renderSelector([appA, appB, appC]);
        const selected = () => getSelectedApps([appA, appB, appC], getApi().getFormState().values);

        fireEvent.click(screen.getByText('all'));
        expect(selected()).toEqual([appA, appB, appC]);

        fireEvent.click(screen.getByText('out of sync'));
        expect(selected()).toEqual([appB, appC]);

        fireEvent.click(screen.getByText('none'));
        expect(selected()).toEqual([]);
    });

    it('keeps lists up to 50 apps unvirtualized', () => {
        const apps = Array.from({length: 50}, (_, i) => newApp(`app-${i}`, 'argocd', 'Synced'));
        renderSelector(apps);
        expect(screen.queryByTestId('virtualized-list')).toBeNull();
        expect(screen.getAllByRole('checkbox')).toHaveLength(50);
    });

    it('virtualizes lists over 50 apps and selects rows by identity', () => {
        const apps = Array.from({length: 60}, (_, i) => newApp(`app-${i}`, 'argocd', 'Synced'));
        const {getApi, rerender} = renderSelector(apps);
        expect(screen.getByTestId('virtualized-list').getAttribute('data-count')).toBe('60');
        expect(screen.getAllByRole('checkbox')).toHaveLength(3);
        expect(screen.getByText('app-59').getAttribute('title')).toBe('app-59');

        fireEvent.click(screen.getByText('all'));
        expect(getSelectedApps(apps, getApi().getFormState().values)).toEqual(apps);
        expect(checkboxOf('app-59').checked).toBe(true);

        fireEvent.click(checkboxOf('app-59'));
        expect(checkboxOf('app-59').checked).toBe(false);
        expect(getApi().getFormState().values.selectedApps).not.toContain('argocd_app-59');
        expect(getSelectedApps(apps, getApi().getFormState().values)).toEqual(apps.slice(0, 59));

        fireEvent.click(screen.getByText('none'));
        fireEvent.click(checkboxOf('app-59'));
        expect(getApi().getFormState().values.selectedApps).toEqual(['argocd_app-59']);

        const prepended = newApp('app-new', 'argocd', 'Synced');
        rerender([prepended, ...apps]);
        expect(checkboxOf('app-59').checked).toBe(true);
        expect(getSelectedApps([prepended, ...apps], getApi().getFormState().values)).toEqual([apps[59]]);
    });
});
