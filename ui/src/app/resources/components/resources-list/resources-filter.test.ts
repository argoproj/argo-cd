import {Resource} from '../../../shared/models';
import {ResourcesListPreferences} from '../../../shared/services';
import {CORE_API_GROUP_FILTER, getFilterResults} from './resources-filter';

const resources = [
    {kind: 'Service', group: ''},
    {kind: 'Deployment', group: 'apps'}
] as Resource[];

const filterByApiGroups = (apiGroupFilter: string[]) => {
    const pref = {} as ResourcesListPreferences;
    ResourcesListPreferences.clearFilters(pref);
    pref.apiGroupFilter = apiGroupFilter;
    return getFilterResults(resources, pref).map(r => r.filterResult.apiGroup);
};

test('<core> matches only resources in the core API group', () => {
    expect(filterByApiGroups([CORE_API_GROUP_FILTER])).toEqual([true, false]);
});

test('<core> can be combined with named API groups', () => {
    expect(filterByApiGroups([CORE_API_GROUP_FILTER, 'apps'])).toEqual([true, true]);
});

describe('getFilterResults kind filter', () => {
    const kinds = [{kind: 'Deployment'}, {kind: 'Pod'}, {kind: 'ConfigMap'}] as Resource[];

    const visibleKinds = (kindFilter: string[]) => {
        const pref = {} as ResourcesListPreferences;
        ResourcesListPreferences.clearFilters(pref);
        pref.kindFilter = kindFilter;
        return getFilterResults(kinds, pref)
            .filter(r => r.filterResult.kind)
            .map(r => r.kind);
    };

    test('shows every kind when nothing is selected', () => {
        expect(visibleKinds([])).toEqual(['Deployment', 'Pod', 'ConfigMap']);
    });

    test('shows only the selected kinds by default', () => {
        expect(visibleKinds(['Pod', 'ConfigMap'])).toEqual(['Pod', 'ConfigMap']);
    });

    test('hides the selected kinds when the filter is set to Exclude', () => {
        expect(visibleKinds(['!', 'Pod', 'ConfigMap'])).toEqual(['Deployment']);
    });

    test('a lone sentinel without selected kinds does not hide anything', () => {
        expect(visibleKinds(['!'])).toEqual(['Deployment', 'Pod', 'ConfigMap']);
    });
});
