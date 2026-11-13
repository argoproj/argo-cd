import {Resource} from '../../../shared/models';
import {ResourcesListPreferences} from '../../../shared/services';
import {getFilterResults} from './resources-filter';

const resource = (kind: string): Resource => ({kind, name: kind.toLowerCase(), namespace: 'default', group: '', version: 'v1'}) as Resource;

const resources = [resource('Deployment'), resource('Pod'), resource('ConfigMap')];

const pref = (kindFilter: string[]): ResourcesListPreferences =>
    ({
        projectsFilter: [],
        syncFilter: [],
        healthFilter: [],
        namespacesFilter: [],
        clustersFilter: [],
        apiGroupFilter: [],
        kindFilter
    }) as ResourcesListPreferences;

const visibleKinds = (kindFilter: string[]) =>
    getFilterResults(resources, pref(kindFilter))
        .filter(r => r.filterResult.kind)
        .map(r => r.kind);

describe('getFilterResults kind filter', () => {
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
