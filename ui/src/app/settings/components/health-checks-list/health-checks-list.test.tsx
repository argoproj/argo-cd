/* eslint-env jest */
declare const test: any;
declare const expect: any;
declare const describe: any;
declare const jest: any;

import * as React from 'react';
import {render, screen, fireEvent} from '@testing-library/react';
import * as models from '../../../shared/models';
import {filterHealthChecks, getHealthCheckFilterResults, HealthChecksListPreferencesHelper} from './health-checks-filter';
import {searchHealthChecks, sortHealthChecks} from './health-checks-list';
import {HealthCheckDetailsPanel} from './health-check-details-panel';

jest.mock('../../../shared/components/monaco-editor', () => ({
    MonacoEditor: (props: any) => (
        <div data-testid='monaco-editor-mock' data-language={props.editor?.input?.language} data-readonly={String(props.editor?.options?.readOnly)}>
            {props.editor?.input?.text}
        </div>
    )
}));

const mockItems: models.HealthCheckItem[] = [
    {group: 'apps', kind: 'Deployment', key: 'apps/Deployment', origin: 'BuiltinGo', isWildcard: false},
    {group: 'argoproj.io', kind: 'Rollout', key: 'argoproj.io/Rollout', origin: 'BuiltinLua', isWildcard: false, luaScript: 'return {}'},
    {group: 'custom.io', kind: 'Widget', key: 'custom.io/Widget', origin: 'CustomLua', isWildcard: false, luaScript: 'hs = {}'},
    {group: 'apps', kind: 'Deployment', key: 'apps/Deployment', origin: 'OverrideLua', isWildcard: false, luaScript: 'return {}'},
    {group: 'cnrm.cloud.google.com', kind: '*', key: '*.cnrm.cloud.google.com', origin: 'BuiltinLua', isWildcard: true}
];

describe('HealthChecks filter, search and sort tests', () => {
    test('filters items by origin', () => {
        const pref = {originFilter: ['BuiltinGo']};
        const results = getHealthCheckFilterResults(mockItems, pref);
        const filtered = filterHealthChecks(results);

        expect(filtered.length).toBe(1);
        expect(filtered[0].key).toBe('apps/Deployment');
        expect(filtered[0].origin).toBe('BuiltinGo');
    });

    test('returns all items when filter is empty', () => {
        const pref = {originFilter: []};
        const results = getHealthCheckFilterResults(mockItems, pref);
        const filtered = filterHealthChecks(results);

        expect(filtered.length).toBe(mockItems.length);
    });

    test('clears filters with helper', () => {
        const pref = {originFilter: ['BuiltinGo', 'CustomLua']};
        HealthChecksListPreferencesHelper.clearFilters(pref);
        expect(pref.originFilter).toEqual([]);
    });

    test('searches health checks by group', () => {
        const results = searchHealthChecks(mockItems, 'argoproj');
        expect(results.length).toBe(1);
        expect(results[0].key).toBe('argoproj.io/Rollout');
    });

    test('searches health checks by kind', () => {
        const results = searchHealthChecks(mockItems, 'Widget');
        expect(results.length).toBe(1);
        expect(results[0].kind).toBe('Widget');
    });

    test('searches health checks by key', () => {
        const results = searchHealthChecks(mockItems, '*.cnrm');
        expect(results.length).toBe(1);
        expect(results[0].key).toBe('*.cnrm.cloud.google.com');
    });

    test('returns empty array when search matches nothing', () => {
        const results = searchHealthChecks(mockItems, 'non-existent-gvk');
        expect(results.length).toBe(0);
    });

    test('returns all items when search text is empty', () => {
        const results = searchHealthChecks(mockItems, '');
        expect(results.length).toBe(mockItems.length);
    });

    test('sorts deterministically by group, then kind, then key', () => {
        const compareString = (a: string, b: string) => a.localeCompare(b);
        const sorted = sortHealthChecks(mockItems, 'group', compareString);

        expect(sorted[0].group).toBe('apps');
        expect(sorted[1].group).toBe('apps');
        expect(sorted[2].group).toBe('argoproj.io');
        expect(sorted[3].group).toBe('cnrm.cloud.google.com');
        expect(sorted[4].group).toBe('custom.io');
    });

    test('sorts by key directly when requested', () => {
        const compareString = (a: string, b: string) => a.localeCompare(b);
        const sorted = sortHealthChecks(mockItems, 'key', compareString);

        expect(sorted[0].key).toBe('*.cnrm.cloud.google.com');
        expect(sorted[1].key).toBe('apps/Deployment');
    });
});

describe('HealthCheckDetailsPanel tests', () => {
    test('renders null item safely without crash', () => {
        const onClose = jest.fn();
        const {container} = render(<HealthCheckDetailsPanel item={null} onClose={onClose} />);
        expect(container.querySelector('.health-check-details-panel__header')).toBeNull();
    });

    test('renders BuiltinGo item metadata and native Go explanation', () => {
        const onClose = jest.fn();
        const item: models.HealthCheckItem = {
            group: 'apps',
            kind: 'Deployment',
            key: 'apps/Deployment',
            origin: 'BuiltinGo',
            isWildcard: false
        };

        render(<HealthCheckDetailsPanel item={item} onClose={onClose} />);

        expect(screen.getAllByText('apps/Deployment').length).toBeGreaterThan(0);
        expect(screen.getByText('BuiltinGo')).toBeInTheDocument();
        expect(screen.getByText(/implemented natively in Go within/i)).toBeInTheDocument();
    });

    test('renders BuiltinLua item metadata and read-only MonacoEditor with lua language', () => {
        const onClose = jest.fn();
        const item: models.HealthCheckItem = {
            group: 'argoproj.io',
            kind: 'Rollout',
            key: 'argoproj.io/Rollout',
            origin: 'BuiltinLua',
            isWildcard: false,
            luaScript: 'return { status = "Healthy" }'
        };

        render(<HealthCheckDetailsPanel item={item} onClose={onClose} />);

        expect(screen.getAllByText('argoproj.io/Rollout').length).toBeGreaterThan(0);
        expect(screen.getByText('BuiltinLua')).toBeInTheDocument();
        expect(screen.getByText('Health Check Source')).toBeInTheDocument();

        const monacoMock = screen.getByTestId('monaco-editor-mock');
        expect(monacoMock).toBeInTheDocument();
        expect(monacoMock).toHaveAttribute('data-language', 'lua');
        expect(monacoMock).toHaveAttribute('data-readonly', 'true');
        expect(monacoMock.textContent).toBe('return { status = "Healthy" }');
    });

    test('calls onClose callback when Close button is clicked', () => {
        const onClose = jest.fn();
        const item: models.HealthCheckItem = {
            group: 'custom.io',
            kind: 'Widget',
            key: 'custom.io/Widget',
            origin: 'CustomLua',
            isWildcard: false,
            luaScript: 'return {}'
        };

        render(<HealthCheckDetailsPanel item={item} onClose={onClose} />);

        const closeBtn = screen.getByText('Close');
        fireEvent.click(closeBtn);
        expect(onClose).toHaveBeenCalledTimes(1);
    });
});
