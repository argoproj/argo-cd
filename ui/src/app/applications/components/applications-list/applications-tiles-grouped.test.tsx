import * as React from 'react';
import {fireEvent, render} from '@testing-library/react';
import type {Application} from '../../../shared/models';
import {VirtualizedGroupedTiles} from './applications-tiles-grouped';
import {buildProjectRows, countByProject} from './project-groups';

jest.mock('react-virtualized/dist/commonjs/AutoSizer', () => ({
    __esModule: true,
    default: ({children}: {children: (size: {width: number}) => React.ReactNode}) => children({width: 1200})
}));
jest.mock('react-virtualized/dist/commonjs/WindowScroller', () => ({
    __esModule: true,
    default: ({children}: {children: (props: object) => React.ReactNode}) => children({height: 600, isScrolling: false, onChildScroll: jest.fn(), scrollTop: 0})
}));
jest.mock('react-virtualized/dist/commonjs/CellMeasurer', () => ({
    ...jest.requireActual('react-virtualized/dist/commonjs/CellMeasurer'),
    __esModule: true,
    default: ({children}: {children: React.ReactNode}) => children
}));
jest.mock('react-virtualized/dist/commonjs/List', () => ({
    __esModule: true,
    default: ({rowCount, rowRenderer}: {rowCount: number; rowRenderer: (p: object) => React.ReactNode}) =>
        require('react').createElement(
            'div',
            null,
            Array.from({length: rowCount}, (_, index) => rowRenderer({index, key: String(index), style: {}, parent: {}}))
        )
}));

const app = (name: string, project: string): Application => ({kind: 'Application', metadata: {name, namespace: 'argocd', uid: `uid-${name}`}, spec: {project}}) as Application;

describe('VirtualizedGroupedTiles', () => {
    // 1200px wide container fits 3 tiles per row
    const apps = [app('a', 'apps'), app('b', 'apps'), app('c', 'infra'), app('d', 'infra'), app('e', 'infra'), app('f', 'infra')];
    const renderGrouped = (collapsed: string[] = [], onToggle = jest.fn()) =>
        render(
            <VirtualizedGroupedTiles
                rows={buildProjectRows(apps, {collapsed, counts: countByProject(apps)})}
                selectedApp={-1}
                layoutKey='k'
                onToggle={onToggle}
                renderTile={a => (
                    <div key={a.metadata.name} data-testid='tile'>
                        {a.metadata.name}
                    </div>
                )}
            />
        );

    it('renders a heading row per project and tile rows that never mix projects', () => {
        const {container} = renderGrouped();
        expect(Array.from(container.querySelectorAll('.project-group-heading__name')).map(el => el.textContent)).toEqual(['apps', 'infra']);
        const tileRows = Array.from(container.querySelectorAll('.applications-tiles__virtual-group-tiles'));
        expect(tileRows.map(row => Array.from(row.querySelectorAll('[data-testid="tile"]')).map(el => el.textContent))).toEqual([['a', 'b'], ['c', 'd', 'e'], ['f']]);
    });

    it('omits tiles of a collapsed project and reports heading clicks', () => {
        const onToggle = jest.fn();
        const {container} = renderGrouped(['infra'], onToggle);
        expect(Array.from(container.querySelectorAll('[data-testid="tile"]')).map(el => el.textContent)).toEqual(['a', 'b']);
        fireEvent.click(container.querySelectorAll('.project-group-heading')[1]);
        expect(onToggle).toHaveBeenCalledWith('infra');
    });
});
