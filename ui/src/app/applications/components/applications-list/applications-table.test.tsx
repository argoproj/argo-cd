import * as React from 'react';
import {fireEvent, render, waitFor} from '@testing-library/react';
import {KeybindingProvider} from 'argo-ui/v2';
import type {Application} from '../../../shared/models';
import {services} from '../../../shared/services';
import {ApplicationsTable} from './applications-table';
import {countByProject, ProjectGrouping} from './project-groups';
import {PROJECT_HEADING_ROW_HEIGHT, TABLE_ROW_HEIGHT} from './virtual-scroll';

jest.mock('./application-table-row', () => ({
    ApplicationTableRow: ({app}: {app: Application}) => require('react').createElement('div', {'data-testid': 'app-row'}, app.metadata.name)
}));
jest.mock('./appset-table-row', () => ({AppSetTableRow: (): null => null}));
jest.mock('react-virtualized/dist/commonjs/AutoSizer', () => ({
    __esModule: true,
    default: ({children}: {children: (size: {width: number}) => React.ReactNode}) => children({width: 1200})
}));
jest.mock('react-virtualized/dist/commonjs/WindowScroller', () => ({
    __esModule: true,
    default: ({children}: {children: (props: object) => React.ReactNode}) => children({height: 600, isScrolling: false, onChildScroll: jest.fn(), scrollTop: 0})
}));

const mockRowHeights: number[] = [];
jest.mock('react-virtualized/dist/commonjs/List', () => ({
    __esModule: true,
    default: ({rowCount, rowHeight, rowRenderer}: {rowCount: number; rowHeight: (p: {index: number}) => number; rowRenderer: (p: object) => React.ReactNode}) => {
        mockRowHeights.length = 0;
        const rows: React.ReactNode[] = [];
        for (let index = 0; index < rowCount; index++) {
            mockRowHeights.push(rowHeight({index}));
            rows.push(rowRenderer({index, key: String(index), style: {}}));
        }
        return require('react').createElement('div', null, rows);
    }
}));

const app = (name: string, project: string): Application =>
    ({
        kind: 'Application',
        metadata: {name, namespace: 'argocd', uid: `uid-${name}`},
        spec: {project, destination: {}, source: {}},
        status: {health: {status: 'Healthy'}, sync: {status: 'Synced'}}
    }) as Application;

const grouping = (apps: Application[], collapsed: string[] = [], onToggle = jest.fn()): ProjectGrouping => ({collapsed, counts: countByProject(apps), onToggle});

const renderTable = (applications: Application[], props: {grouping?: ProjectGrouping; useVirtualScrolling?: boolean} = {}) =>
    render(
        <KeybindingProvider>
            <ApplicationsTable applications={applications} syncApplication={jest.fn()} refreshApplication={jest.fn()} deleteApplication={jest.fn()} {...props} />
        </KeybindingProvider>
    );

const sequence = (container: HTMLElement) =>
    Array.from(container.querySelectorAll('.project-group-heading__name, [data-testid="app-row"]')).map(el =>
        el.classList.contains('project-group-heading__name') ? `#${el.textContent}` : el.textContent
    );

describe('ApplicationsTable grouping', () => {
    const apps = [app('a', 'apps'), app('c', 'apps'), app('b', 'infra')];

    beforeAll(() => services.viewPreferences.init());

    it('renders no headings without grouping', async () => {
        const {container} = renderTable(apps);
        await waitFor(() => expect(sequence(container)).toEqual(['a', 'c', 'b']));
    });

    it('renders a heading before each project', async () => {
        const {container} = renderTable(apps, {grouping: grouping(apps)});
        await waitFor(() => expect(sequence(container)).toEqual(['#apps', 'a', 'c', '#infra', 'b']));
    });

    it('hides rows of a collapsed project and reports heading clicks', async () => {
        const onToggle = jest.fn();
        const {container} = renderTable(apps, {grouping: grouping(apps, ['apps'], onToggle)});
        await waitFor(() => expect(sequence(container)).toEqual(['#apps', '#infra', 'b']));
        fireEvent.click(container.querySelectorAll('.project-group-heading')[1]);
        expect(onToggle).toHaveBeenCalledWith('infra');
    });

    it('gives headings their own row height when virtualized', async () => {
        const many = Array.from({length: 60}, (_, i) => app(`app-${String(i).padStart(2, '0')}`, i < 30 ? 'apps' : 'infra'));
        const {container} = renderTable(many, {grouping: grouping(many), useVirtualScrolling: true});
        await waitFor(() => expect(container.querySelectorAll('.project-group-heading')).toHaveLength(2));
        expect(mockRowHeights).toHaveLength(62);
        expect(mockRowHeights[0]).toBe(PROJECT_HEADING_ROW_HEIGHT);
        expect(mockRowHeights[1]).toBe(TABLE_ROW_HEIGHT);
        expect(mockRowHeights[31]).toBe(PROJECT_HEADING_ROW_HEIGHT);
    });
});
