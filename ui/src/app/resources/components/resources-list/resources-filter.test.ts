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
