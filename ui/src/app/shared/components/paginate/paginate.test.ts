import {groupItems} from './paginate';

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
