import * as React from 'react';
import * as renderer from 'react-test-renderer';

jest.mock('argo-ui', () => ({
    Tooltip: ({content, children}: {content: React.ReactNode; children: React.ReactNode}) => (
        <span data-test-tooltip={typeof content === 'string' ? content : ''}>{children}</span>
    )
}));

import {EventsList} from './events-list';
import * as models from '../../models';

function makeEvent(overrides: Partial<models.Event> = {}): models.Event {
    return {
        metadata: {uid: 'uid-1', name: 'evt', namespace: 'default'} as models.ObjectMeta,
        involvedObject: {kind: 'Pod', namespace: 'default', name: 'p', uid: 'p-uid', apiVersion: 'v1', resourceVersion: '1', fieldPath: ''},
        reason: 'FailedScheduling',
        message: 'no nodes available',
        source: {component: '', host: ''},
        firstTimestamp: '2025-01-01T00:00:00Z',
        lastTimestamp: '2025-01-01T00:01:00Z',
        count: 1,
        type: 'Warning',
        eventTime: '2025-01-01T00:00:00Z',
        series: {count: 0, lastObservedTime: '', state: ''},
        action: '',
        related: {kind: '', namespace: '', name: '', uid: '', apiVersion: '', resourceVersion: '', fieldPath: ''},
        reportingComponent: '',
        reportingInstance: '',
        ...overrides
    };
}

function tooltipContents(inst: renderer.ReactTestInstance): string[] {
    return inst.findAll(node => node.type === 'span' && node.props && typeof node.props['data-test-tooltip'] === 'string' && node.props['data-test-tooltip'] !== '').map(n => n.props['data-test-tooltip']);
}

describe('EventsList', () => {
    it('renders a no-events message when given an empty list', () => {
        const inst = renderer.create(<EventsList events={[]} />).root;
        expect(inst.findAllByType('p').length).toBe(1);
    });

    it('renders reportingComponent in a tooltip on the reason cell', () => {
        const event = makeEvent({reportingComponent: 'cert-manager'});
        const inst = renderer.create(<EventsList events={[event]} />).root;
        expect(tooltipContents(inst)).toEqual(['Reported by: cert-manager']);
    });

    it('falls back to source.component when reportingComponent is empty', () => {
        const event = makeEvent({reportingComponent: '', source: {component: 'kubelet', host: 'node-1'}});
        const inst = renderer.create(<EventsList events={[event]} />).root;
        expect(tooltipContents(inst)).toEqual(['Reported by: kubelet']);
    });

    it('omits the tooltip when both reportingComponent and source.component are empty', () => {
        const event = makeEvent({reportingComponent: '', source: {component: '', host: ''}});
        const inst = renderer.create(<EventsList events={[event]} />).root;
        expect(tooltipContents(inst)).toEqual([]);
    });

    it('prefers reportingComponent over source.component', () => {
        const event = makeEvent({reportingComponent: 'gateway-controller', source: {component: 'kubelet', host: 'node-1'}});
        const inst = renderer.create(<EventsList events={[event]} />).root;
        expect(tooltipContents(inst)).toEqual(['Reported by: gateway-controller']);
    });
});
