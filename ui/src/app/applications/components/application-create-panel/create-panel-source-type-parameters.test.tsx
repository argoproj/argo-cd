import {Form, FormApi} from 'argo-ui';
import {act, fireEvent, render, screen, waitFor} from '@testing-library/react';
import * as React from 'react';
import * as models from '../../../shared/models';
import {Context} from '../../../shared/context';
import {services} from '../../../shared/services';
import {CreatePanelSourceTypeParameters} from './create-panel-source-type-parameters';

jest.mock('../../../shared/services', () => ({
    services: {
        repos: {
            appDetails: jest.fn()
        }
    }
}));

jest.mock('lodash-es', () => ({
    cloneDeep: (value: unknown) => JSON.parse(JSON.stringify(value))
}));

function applicationWithSources(sources: models.ApplicationSource[]): models.Application {
    return {
        metadata: {name: 'sandbox'},
        spec: {
            project: 'default',
            destination: {server: 'https://kubernetes.default.svc', namespace: 'sandbox'},
            sources
        }
    } as models.Application;
}

describe('CreatePanelSourceTypeParameters', () => {
    const appDetails = services.repos.appDetails as jest.Mock;

    beforeEach(() => {
        appDetails.mockReset();
    });

    test('stores an external Helm value file on the chart source', async () => {
        const app = applicationWithSources([
            {repoURL: 'https://prometheus-community.github.io/helm-charts', chart: 'prometheus', targetRevision: '15.7.1'} as models.ApplicationSource,
            {repoURL: 'https://git.example.com/org/value-files.git', targetRevision: 'main', ref: 'values'} as models.ApplicationSource
        ]);
        appDetails.mockResolvedValue({
            type: 'Helm',
            path: '',
            helm: {name: 'prometheus', valueFiles: ['values.yaml'], parameters: [], fileParameters: []}
        } as models.RepoAppDetails);
        let formApi: FormApi | undefined;
        const onSubmit = jest.fn();

        render(
            <Context.Provider value={{notifications: {show: jest.fn()}} as any}>
                <Form defaultValues={app} getApi={api => (formApi = api)} onSubmit={onSubmit}>
                    {api => <CreatePanelSourceTypeParameters formApi={api} sourceIndex={0} />}
                </Form>
            </Context.Provider>
        );

        await screen.findByText('VALUES FILES');
        const valueFilesInput = document.querySelector('.tags-input input') as HTMLInputElement;
        const valueFile = '$values/charts/prometheus/values.yaml';
        fireEvent.change(valueFilesInput, {target: {value: valueFile}});
        fireEvent.keyUp(valueFilesInput, {key: 'Enter', keyCode: 13});

        await waitFor(() => expect(formApi?.values.spec.sources[0].helm.valueFiles).toEqual([valueFile]));
        expect(formApi?.values.spec.sources[1].helm).toBeUndefined();
        expect(formApi?.values.spec.sources[1].ref).toBe('values');

        act(() => formApi?.submitForm(null));
        expect(onSubmit).toHaveBeenCalledWith(
            expect.objectContaining({
                spec: expect.objectContaining({
                    sources: expect.arrayContaining([expect.objectContaining({helm: expect.objectContaining({valueFiles: [valueFile]})})])
                })
            }),
            null,
            expect.anything()
        );
    });

    test('does not show generator parameters or discover a ref-only source', () => {
        const app = applicationWithSources([
            {repoURL: 'https://prometheus-community.github.io/helm-charts', chart: 'prometheus', targetRevision: '15.7.1'} as models.ApplicationSource,
            {repoURL: 'https://git.example.com/org/value-files.git', targetRevision: 'main', ref: 'values'} as models.ApplicationSource
        ]);

        const {container} = render(
            <Context.Provider value={{notifications: {show: jest.fn()}} as any}>
                <Form defaultValues={app}>{api => <CreatePanelSourceTypeParameters formApi={api} sourceIndex={1} />}</Form>
            </Context.Provider>
        );

        expect(container).toBeEmptyDOMElement();
        expect(appDetails).not.toHaveBeenCalled();
    });
});
