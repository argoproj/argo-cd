import {render, screen} from '@testing-library/react';
import * as React from 'react';

import * as models from '../../../shared/models';

// lodash-es ships as untransformed ESM under node_modules and is not covered by the jest
// transform config, so map the single helper this module uses to a CJS-friendly stub.
jest.mock('lodash-es', () => ({
    __esModule: true,
    cloneDeep: (value: any) => (value == null ? value : JSON.parse(JSON.stringify(value)))
}));

// eslint-disable-next-line import/first
import {ApplicationParameters} from './application-parameters';

// Paginate pulls in a view-preferences DataLoader that is irrelevant to what we assert here,
// so replace it with a passthrough that simply renders its children for the provided data.
jest.mock('../../../shared/components', () => ({
    ...jest.requireActual('../../../shared/components'),
    Paginate: ({data, children}: {data: any[]; children: (d: any[]) => React.ReactNode}) => <>{children(data)}</>
}));

// The expanded per-source panel and the add-source panel are not exercised by the collapsed
// summary and depend on backend services, so stub them out.
jest.mock('./application-parameters-source', () => ({ApplicationParametersSource: () => null}));
jest.mock('./source-panel', () => ({SourcePanel: () => null}));

const multiSourceApp = (): models.Application =>
    ({
        metadata: {name: 'test-app'},
        spec: {
            project: 'default',
            sources: [
                {
                    repoURL: 'https://github.com/example/repo',
                    targetRevision: 'main',
                    plugin: {
                        name: 'my-plugin',
                        env: [
                            {name: 'FOO', value: 'bar'},
                            {name: 'ENVIRONMENT', value: 'prod'}
                        ]
                    }
                }
            ]
        }
    }) as models.Application;

test('collapsed multi-source summary shows plugin env values', () => {
    render(<ApplicationParameters application={multiSourceApp()} collapsedSources={[true]} handleCollapse={() => undefined} />);

    expect(screen.getByText(/ENV=FOO=bar ENVIRONMENT=prod/)).toBeInTheDocument();
});
