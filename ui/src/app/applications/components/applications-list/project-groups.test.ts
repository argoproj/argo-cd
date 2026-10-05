import {renderHook} from '@testing-library/react';
import {Application} from '../../../shared/models';
import {appProject, buildProjectRows, buildTileRows, countByProject, useResetSelectionOnIdentityChange, visibleApps} from './project-groups';

const app = (name: string, project?: string): Application => ({kind: 'Application', metadata: {name, namespace: 'argocd', uid: `uid-${name}`}, spec: {project}}) as Application;

const describeRows = (rows: ReturnType<typeof buildProjectRows>) =>
    rows.map(row => (row.kind === 'heading' ? `#${row.project}(${row.count})${row.collapsed ? '-' : '+'}` : `${row.app.metadata.name}@${row.index}`));

describe('project-groups', () => {
    const apps = [app('a', 'apps'), app('c', 'apps'), app('b', 'infra')];

    it('falls back to the default project when spec.project is empty', () => {
        expect(appProject(app('x'))).toBe('default');
        expect(appProject(app('x', ''))).toBe('default');
    });

    it('returns plain app rows when grouping is off', () => {
        expect(describeRows(buildProjectRows(apps))).toEqual(['a@0', 'c@1', 'b@2']);
    });

    it('puts a heading before each project and numbers apps continuously', () => {
        const rows = buildProjectRows(apps, {collapsed: [], counts: countByProject(apps)});
        expect(describeRows(rows)).toEqual(['#apps(2)+', 'a@0', 'c@1', '#infra(1)+', 'b@2']);
    });

    it('hides apps of a collapsed project and numbers only visible apps', () => {
        const rows = buildProjectRows(apps, {collapsed: ['apps'], counts: countByProject(apps)});
        expect(describeRows(rows)).toEqual(['#apps(2)-', '#infra(1)+', 'b@0']);
        expect(visibleApps(rows).map(a => a.metadata.name)).toEqual(['b']);
    });

    it('ignores collapsed names that match no project', () => {
        const rows = buildProjectRows(apps, {collapsed: ['gone'], counts: countByProject(apps)});
        expect(describeRows(rows)).toEqual(['#apps(2)+', 'a@0', 'c@1', '#infra(1)+', 'b@2']);
    });

    it('shows the total count of a project, not the page slice', () => {
        const all = [...apps, app('d', 'infra'), app('e', 'infra')];
        const page = all.slice(0, 3);
        const rows = buildProjectRows(page, {collapsed: [], counts: countByProject(all)});
        expect(describeRows(rows)).toContain('#infra(3)+');
    });

    it('chunks tiles per project and never mixes projects in one row', () => {
        const many = [app('a', 'apps'), app('b', 'apps'), app('c', 'apps'), app('d', 'infra')];
        const tileRows = buildTileRows(buildProjectRows(many, {collapsed: [], counts: countByProject(many)}), 2);
        expect(tileRows.map(row => (row.kind === 'heading' ? `#${row.project}` : row.items.map(i => i.app.metadata.name).join(',')))).toEqual(['#apps', 'a,b', 'c', '#infra', 'd']);
    });
});

describe('useResetSelectionOnIdentityChange', () => {
    const run = (initial: Application[], selected: number) => {
        const reset = jest.fn();
        const hook = renderHook(({apps, sel}) => useResetSelectionOnIdentityChange(apps, sel, reset), {initialProps: {apps: initial, sel: selected}});
        return {reset, hook};
    };

    it('clears the selection when the same index now holds a different app', () => {
        const {reset, hook} = run([app('a'), app('b'), app('c')], 1);
        hook.rerender({apps: [app('c'), app('d'), app('e')], sel: 1});
        expect(reset).toHaveBeenCalledTimes(1);
    });

    it('keeps the selection when the app at the index is unchanged', () => {
        const {reset, hook} = run([app('a'), app('b')], 1);
        hook.rerender({apps: [app('a'), app('b')], sel: 1});
        expect(reset).not.toHaveBeenCalled();
    });

    it('does not reset when only the selection moved', () => {
        const {reset, hook} = run([app('a'), app('b')], 0);
        hook.rerender({apps: [app('a'), app('b')], sel: 1});
        expect(reset).not.toHaveBeenCalled();
    });

    it('ignores the no-selection state', () => {
        const {reset, hook} = run([app('a')], -1);
        hook.rerender({apps: [app('b')], sel: -1});
        expect(reset).not.toHaveBeenCalled();
    });
});
