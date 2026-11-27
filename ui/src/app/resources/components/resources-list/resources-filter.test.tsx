import * as React from 'react';
import {render} from '@testing-library/react';
import '@testing-library/jest-dom';
import {getFilterResults, ResourcesFilter} from './resources-filter';
import {resources} from '../../../applications/components/resources';
import {Resource} from '../../../shared/models';
import {ResourcesListPreferences} from '../../../shared/services';

const mockFilter = jest.fn();
jest.mock('../../../applications/components/filter/filter', () => ({
    FiltersGroup: ({children}: {children: React.ReactNode}) => <div>{children}</div>,
    Filter: (props: any) => {
        mockFilter(props);
        return <div data-testid={`filter-${props.label}`}>{props.label}</div>;
    }
}));

describe('ResourcesFilter', () => {
    beforeEach(() => {
        mockFilter.mockClear();
    });

    test('passes kind abbreviations to KINDS filter for shorthand matching (#31126)', () => {
        const dummyApps = [
            {
                kind: 'Pod',
                name: 'pod-1',
                namespace: 'default',
                status: 'Synced',
                health: {status: 'Healthy'}
            }
        ] as unknown as Resource[];

        const pref = {
            projectsFilter: [],
            namespacesFilter: [],
            clustersFilter: [],
            syncFilter: [],
            healthFilter: [],
            kindFilter: [],
            apiGroupFilter: [],
            hideFilters: false
        } as unknown as ResourcesListPreferences;

        const filterResults = getFilterResults(dummyApps, pref);

        render(<ResourcesFilter apps={filterResults} pref={pref} onChange={jest.fn()} />);

        const kindFilterCall = mockFilter.mock.calls.find(call => call[0].label === 'KINDS');
        expect(kindFilterCall).toBeDefined();
        expect(kindFilterCall[0].field).toBe(true);
        expect(kindFilterCall[0].abbreviations).toBe(resources);
        expect(kindFilterCall[0].abbreviations.get('Pod')).toBe('pod');
        expect(kindFilterCall[0].abbreviations.get('Service')).toBe('svc');
    });
});
