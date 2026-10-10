import * as React from 'react';
import {fireEvent, render, screen} from '@testing-library/react';
import '@testing-library/jest-dom';
import {of} from 'rxjs';

import * as models from '../../../shared/models';

// applications/components/utils pulls in ESM-only deps (lodash-es) that jest does not
// transform, so mock it with a plain nodeKey.
jest.mock('../utils', () => ({
    nodeKey: (n: {group: string; kind: string; namespace: string; name: string}) => [n.group, n.kind, n.namespace, n.name].join('/')
}));

jest.mock('../../../shared/services', () => ({
    services: {
        viewPreferences: {
            getPreferences: () => of({appDetails: {compactDiff: false, inlineDiff: true}}),
            updatePreferences: jest.fn()
        }
    }
}));

import {ApplicationResourcesDiff} from './application-resources-diff';

const changed = (kind: string, name: string): models.ResourceDiff =>
    ({
        group: 'apps',
        kind,
        namespace: 'default',
        name,
        hook: false,
        normalizedLiveState: {spec: {replicas: 1}},
        predictedLiveState: {spec: {replicas: 2}}
    }) as unknown as models.ResourceDiff;

describe('ApplicationResourcesDiff', () => {
    const a = changed('Deployment', 'a');
    const b = changed('Deployment', 'b');

    it('names each resource checkbox after its resource', () => {
        render(<ApplicationResourcesDiff states={[a, b]} onSync={jest.fn()} />);

        expect(screen.getByLabelText('Select apps/Deployment/default/a to sync')).toBeInTheDocument();
        expect(screen.getByLabelText('Select apps/Deployment/default/b to sync')).toBeInTheDocument();
    });

    it('does not sync a checked resource that is no longer in the diff', () => {
        const onSync = jest.fn();
        const {rerender} = render(<ApplicationResourcesDiff states={[a, b]} onSync={onSync} />);

        fireEvent.click(screen.getByLabelText('Select apps/Deployment/default/a to sync'));
        expect(screen.getByRole('button', {name: 'Sync selected (1)'})).toBeEnabled();

        // the diff reloads without resource a
        rerender(<ApplicationResourcesDiff states={[b]} onSync={onSync} />);
        expect(screen.getByRole('button', {name: 'Sync selected'})).toBeDisabled();

        fireEvent.click(screen.getByLabelText('Select apps/Deployment/default/b to sync'));
        fireEvent.click(screen.getByRole('button', {name: 'Sync selected (1)'}));
        expect(onSync).toHaveBeenCalledWith(['apps/Deployment/default/b']);
    });

    it('shows no resource checkboxes without onSync', () => {
        render(<ApplicationResourcesDiff states={[a, b]} />);

        expect(screen.queryByLabelText('Select apps/Deployment/default/a to sync')).not.toBeInTheDocument();
        expect(screen.queryByRole('button', {name: /Sync selected/})).not.toBeInTheDocument();
    });
});
