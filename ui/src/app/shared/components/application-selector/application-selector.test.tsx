import * as React from 'react';
import {fireEvent, render, screen} from '@testing-library/react';
import {Form, FormApi} from 'argo-ui';
import * as models from '../../models';
import {ApplicationSelector, getSelectedApps} from './application-selector';

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
});
