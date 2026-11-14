import * as React from 'react';
import {fireEvent, render, screen} from '@testing-library/react';
import '@testing-library/jest-dom';

import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ApplicationResourcesDiff} from './application-resources-diff';

jest.mock('../../../shared/services', () => {
    const {ViewPreferencesService} = jest.requireActual('../../../shared/services/view-preferences-service');
    return {services: {viewPreferences: new ViewPreferencesService()}};
});

// Enough labels that the first one is far from the changed replicas line.
const labels = Object.fromEntries(Array.from({length: 10}, (_, i) => [`label-${i}`, `value-${i}`]));
const deployment = (replicas: number) => ({apiVersion: 'apps/v1', kind: 'Deployment', metadata: {name: 'guestbook-ui', labels}, spec: {replicas}});

const changed = {group: 'apps', kind: 'Deployment', namespace: 'guestbook', name: 'guestbook-ui', normalizedLiveState: deployment(1), predictedLiveState: deployment(3)};
const unchanged = {group: '', kind: 'Service', namespace: 'guestbook', name: 'guestbook-ui', normalizedLiveState: {kind: 'Service'}, predictedLiveState: {kind: 'Service'}};
const hook = {...changed, name: 'migrate', hook: true};

const renderDiff = (states: Partial<models.ResourceDiff>[]) => render(<ApplicationResourcesDiff states={states as models.ResourceDiff[]} />);

describe('ApplicationResourcesDiff', () => {
    beforeEach(() => {
        window.localStorage.clear();
        services.viewPreferences.init();
        // init() keeps the in-memory preferences after the first test, so reset the diff options explicitly.
        services.viewPreferences.updatePreferences({appDetails: {...services.viewPreferences.getCurrentPreferences().appDetails, compactDiff: false, inlineDiff: false}});
    });

    it('shows changed resources only, skipping unchanged ones and hooks', () => {
        const {container} = renderDiff([changed, unchanged, hook]);

        expect(screen.getByText('apps/Deployment/guestbook/guestbook-ui')).toBeInTheDocument();
        expect(screen.queryByText('/Service/guestbook/guestbook-ui')).not.toBeInTheDocument();
        expect(screen.queryByText('apps/Deployment/guestbook/migrate')).not.toBeInTheDocument();
        expect(container).toHaveTextContent('replicas: 1');
        expect(container).toHaveTextContent('replicas: 3');
    });

    it('hides unchanged lines and remembers it when Compact diff is ticked', () => {
        const {container} = renderDiff([changed]);
        expect(container).toHaveTextContent('label-0: value-0');

        fireEvent.click(screen.getByLabelText('Compact diff'));

        expect(container).not.toHaveTextContent('label-0: value-0');
        expect(container).toHaveTextContent('replicas: 3');
        expect(services.viewPreferences.getCurrentPreferences().appDetails.compactDiff).toBe(true);
    });

    it('switches between split and unified views with Inline diff', () => {
        const {container} = renderDiff([changed]);
        expect(container.querySelector('.diff-split')).toBeInTheDocument();

        fireEvent.click(screen.getByLabelText('Inline diff'));

        expect(container.querySelector('.diff-unified')).toBeInTheDocument();
        expect(container.querySelector('.diff-split')).not.toBeInTheDocument();
        expect(services.viewPreferences.getCurrentPreferences().appDetails.inlineDiff).toBe(true);
    });
});
