import {collapseGroups, groupItems, paginateItems} from './paginate';

describe('groupItems', () => {
    const item = (name: string, project: string) => ({name, project});
    const names = (items: {name: string}[]) => items.map(i => i.name);

    it('orders groups A-Z and keeps the incoming order inside each group', () => {
        // incoming order is "name descending"
        const sorted = [item('z', 'infra'), item('c', 'apps'), item('b', 'infra'), item('a', 'apps')];
        expect(names(groupItems(sorted, i => i.project))).toEqual(['c', 'a', 'z', 'b']);
    });

    it('does not mutate its input', () => {
        const input = [item('b', 'infra'), item('a', 'apps')];
        groupItems(input, i => i.project);
        expect(names(input)).toEqual(['b', 'a']);
    });
});

describe('collapseGroups', () => {
    const item = (name: string, project: string) => ({name, project});
    const names = (items: {name: string}[]) => items.map(i => i.name);
    const project = (i: {project: string}) => i.project;

    it('keeps one placeholder per collapsed group and every item of expanded groups', () => {
        const grouped = [item('a1', 'apps'), item('a2', 'apps'), item('b1', 'infra'), item('b2', 'infra')];
        const result = collapseGroups(grouped, project, g => g === 'apps');
        expect(names(result.items)).toEqual(['a1', 'b1', 'b2']);
        expect(names([...result.placeholders])).toEqual(['a1']);
    });

    it('returns everything when no group is collapsed', () => {
        const grouped = [item('a1', 'apps'), item('b1', 'infra')];
        const result = collapseGroups(grouped, project, () => false);
        expect(names(result.items)).toEqual(['a1', 'b1']);
        expect(result.placeholders.size).toBe(0);
    });

    it('does not mutate its input', () => {
        const grouped = [item('a1', 'apps'), item('a2', 'apps')];
        collapseGroups(grouped, project, () => true);
        expect(names(grouped)).toEqual(['a1', 'a2']);
    });
});

describe('paginateItems', () => {
    const item = (name: string, project: string) => ({name, project});
    const names = (pages: {name: string}[][]) => pages.map(page => page.map(i => i.name));
    const project = (i: {project: string}) => i.project;
    const paginate = (apps: {name: string; project: string}[], pageSize: number, collapsed: string[]) => {
        const {items, placeholders} = collapseGroups(groupItems(apps, project), project, g => collapsed.includes(g));
        return paginateItems(items, pageSize, placeholders);
    };

    it('chunks by page size when nothing is collapsed', () => {
        const apps = ['a', 'b', 'c', 'd', 'e'].map(n => item(n, 'p'));
        expect(names(paginateItems(apps, 2, new Set()))).toEqual([['a', 'b'], ['c', 'd'], ['e']]);
    });

    it('does not count collapsed headings towards the page size', () => {
        const apps = [item('a1', 'a'), item('a2', 'a'), item('b1', 'b'), ...Array.from({length: 9}, (_, i) => item(`c${i}`, 'c'))];
        const pages = names(paginate(apps, 10, ['a', 'b']));
        expect(pages).toHaveLength(1);
        expect(pages[0]).toHaveLength(11);
    });

    it('fills each page with page-size visible apps', () => {
        const apps = [item('a1', 'a'), item('b1', 'b'), ...Array.from({length: 12}, (_, i) => item(`c${i}`, 'c'))];
        const pages = paginate(apps, 10, ['a', 'b']);
        expect(pages.map(page => page.filter(i => i.project === 'c').length)).toEqual([10, 2]);
    });

    it('does not spend pages on a large collapsed group', () => {
        const apps = [...Array.from({length: 100}, (_, i) => item(`a${i}`, 'apps')), item('b1', 'infra'), item('b2', 'infra')];
        expect(names(paginate(apps, 10, []))).toHaveLength(11);
        expect(names(paginate(apps, 10, ['apps']))).toEqual([['a0', 'b1', 'b2']]);
    });

    it('moves a heading to the next page when the current page is full', () => {
        const apps = [...Array.from({length: 10}, (_, i) => item(`a${i}`, 'a')), item('b1', 'b'), item('c1', 'c')];
        expect(names(paginate(apps, 10, ['b']))[1]).toEqual(['b1', 'c1']);
    });

    it('keeps a trailing collapsed heading on the last page', () => {
        const apps = [...Array.from({length: 10}, (_, i) => item(`a${i}`, 'a')), item('z1', 'z')];
        expect(names(paginate(apps, 10, ['z']))).toHaveLength(1);
    });

    it('returns no pages for no items', () => {
        expect(paginateItems([], 10, new Set())).toEqual([]);
    });
});
