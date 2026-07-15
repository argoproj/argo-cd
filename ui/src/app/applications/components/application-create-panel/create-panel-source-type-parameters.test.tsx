import {Form} from 'argo-ui';
import {fireEvent, render, screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
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

    test('stores a typed external Helm value file when Create is clicked without pressing Enter', async () => {
        const app = applicationWithSources([
            {repoURL: 'https://prometheus-community.github.io/helm-charts', chart: 'prometheus', targetRevision: '15.7.1'} as models.ApplicationSource,
            {repoURL: 'https://git.example.com/org/value-files.git', targetRevision: 'main', ref: 'values'} as models.ApplicationSource
        ]);
        appDetails.mockResolvedValue({
            type: 'Helm',
            path: '',
            helm: {name: 'prometheus', valueFiles: ['values.yaml'], parameters: [], fileParameters: []}
        } as models.RepoAppDetails);
        const onSubmit = jest.fn();
        const user = userEvent.setup();

        render(
            <Context.Provider value={{notifications: {show: jest.fn()}} as any}>
                <Form defaultValues={app} onSubmit={onSubmit}>
                    {api => (
                        <>
                            <CreatePanelSourceTypeParameters formApi={api} sourceIndex={0} />
                            <button type='button' onClick={api.submitForm}>
                                Create
                            </button>
                        </>
                    )}
                </Form>
            </Context.Provider>
        );

        await screen.findByText('VALUES FILES');
        const valueFilesInput = document.querySelector('.tags-input input') as HTMLInputElement;
        expect(valueFilesInput).toHaveAttribute('placeholder', '$<ref>/path/to/values.yaml');
        const valueFile = '$values/charts/prometheus/values.yaml';
        fireEvent.change(valueFilesInput, {target: {value: valueFile}});
        await user.click(screen.getByRole('button', {name: 'Create'}));

        await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1));
        const submittedApp = onSubmit.mock.calls[0][0] as models.Application;
        expect(submittedApp.spec.sources?.[0].helm?.valueFiles).toEqual([valueFile]);
        expect(submittedApp.spec.sources?.[1].ref).toBe('values');
        expect(submittedApp.spec.sources?.[1].helm).toBeUndefined();
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
