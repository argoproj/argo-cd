import {render} from '@testing-library/react';
import * as React from 'react';
import {MemoryRouter} from 'react-router-dom';

import {Provider} from '../../../shared/context';
import * as models from '../../../shared/models';
import {ApplicationSummary} from './application-summary';

// application-retry-options.tsx pulls in the ESM-only `lodash-es`, which this repo's jest config
// doesn't transform (unrelated to the description field) - stub it out so requiring
// ApplicationSummary doesn't drag that transitive import chain into the test.
jest.mock('../application-retry-options/application-retry-options', () => ({ApplicationRetryOptions: () => null}));
jest.mock('../application-retry-view/application-retry-view', () => ({ApplicationRetryView: () => null}));

const baseApp = (description?: string) =>
    ({
        metadata: {name: 'guestbook', namespace: 'argocd', labels: {}, annotations: {}},
        spec: {
            project: 'default',
            description,
            destination: {server: 'https://kubernetes.default.svc', namespace: 'default'},
            source: {repoURL: 'https://github.com/example/guestbook.git', path: 'guestbook', targetRevision: 'HEAD'}
        },
        status: {health: {status: 'Healthy'}, sync: {status: 'Synced'}, summary: {}}
    }) as unknown as models.Application;

const ctx = {baseHref: '/', navigation: {goto: jest.fn()}, notifications: {show: jest.fn()}, popup: {}, history: {} as History};

const renderSummary = (app: models.Application) =>
    render(
        <MemoryRouter>
            <Provider value={ctx}>
                <ApplicationSummary app={app} updateApp={jest.fn()} />
            </Provider>
        </MemoryRouter>
    );

test('shows a DESCRIPTION row with the application description in view mode', () => {
    const {container} = renderSummary(baseApp('Guestbook example application'));

    expect(container.textContent).toContain('DESCRIPTION');
    expect(container.textContent).toContain('Guestbook example application');
});

test('omits the DESCRIPTION row in view mode when no description is set', () => {
    const {container} = renderSummary(baseApp(undefined));

    // EditablePanel filters out items with a falsy `view` in read-only mode, so an empty
    // description means the whole row (title included) is absent, not just the value.
    expect(container.textContent).not.toContain('DESCRIPTION');
});
