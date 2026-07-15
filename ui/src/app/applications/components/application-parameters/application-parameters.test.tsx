import {render, screen, waitFor} from '@testing-library/react';
import * as React from 'react';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {Context} from '../../../shared/context';
import {ApplicationParameters} from './application-parameters';

jest.mock('../../../shared/services', () => ({
    services: {
        repos: {
            appDetails: jest.fn()
        },
        viewPreferences: {
            getPreferences: jest.fn().mockResolvedValue({pageSizes: {'5': 10}, sortOptions: {}}),
            updatePreferences: jest.fn()
        }
    }
}));

jest.mock('lodash-es', () => ({
    cloneDeep: (value: unknown) => JSON.parse(JSON.stringify(value))
}));

jest.mock('./source-panel', () => ({
    SourcePanel: () => null
}));

jest.mock('../utils', () => ({
    deleteSourceAction: jest.fn(),
    getAppDefaultSource: (app: models.Application) => app.spec.source || app.spec.sources?.[0],
    getAppDrySource: (app: models.Application) => app.spec.source,
    helpTip: () => null
}));

function applicationWithValuesSource(valuesSource: models.ApplicationSource): models.Application {
    return {
        metadata: {name: 'sandbox'},
        spec: {
            project: 'default',
            destination: {server: 'https://kubernetes.default.svc', namespace: 'sandbox'},
            sources: [{repoURL: 'https://prometheus-community.github.io/helm-charts', chart: 'prometheus', targetRevision: '15.7.1'} as models.ApplicationSource, valuesSource]
        }
    } as models.Application;
}

function renderParameters(app: models.Application) {
    return render(
        <Context.Provider value={{notifications: {show: jest.fn()}} as any}>
            <ApplicationParameters application={app} pageNumber={0} setPageNumber={jest.fn()} collapsedSources={[true, false]} handleCollapse={jest.fn()} appContext={{} as any} />
        </Context.Provider>
    );
}

describe('ApplicationParameters ref-only sources', () => {
    const appDetails = services.repos.appDetails as jest.Mock;

    beforeEach(() => {
        appDetails.mockReset();
    });

    test('shows only core fields without running source discovery', async () => {
        renderParameters(
            applicationWithValuesSource({
                repoURL: 'https://git.example.com/org/value-files.git',
                targetRevision: 'main',
                ref: 'values'
            } as models.ApplicationSource)
        );

        expect(await screen.findByText('Source 2: REF=values, URL=https://git.example.com/org/value-files.git, TARGET REVISION=main')).toBeInTheDocument();
        expect(screen.getByText('REF')).toBeInTheDocument();
        expect(screen.getByText('values')).toBeInTheDocument();
        expect(screen.queryByText('PLUGIN')).not.toBeInTheDocument();
        expect(screen.queryByText('DIRECTORY')).not.toBeInTheDocument();
        expect(appDetails).not.toHaveBeenCalled();
    });

    test('still discovers a source that has both ref and path', async () => {
        appDetails.mockResolvedValue({type: 'Plugin', path: 'manifests', plugin: {name: 'example', env: []}} as models.RepoAppDetails);
        const source = {
            repoURL: 'https://git.example.com/org/value-files.git',
            targetRevision: 'main',
            path: 'manifests',
            ref: 'values'
        } as models.ApplicationSource;

        renderParameters(applicationWithValuesSource(source));

        await waitFor(() => expect(appDetails).toHaveBeenCalledWith(source, 'sandbox', 'default', 1, 0));
        expect(await screen.findByText('PLUGIN')).toBeInTheDocument();
        expect(screen.getByText(/Source 2: TYPE=Plugin/)).toBeInTheDocument();
    });
});
