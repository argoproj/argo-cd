/* eslint-env jest */
declare const test: any;
declare const expect: any;
import * as React from 'react';
import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {fireEvent, render, screen, waitFor} from '@testing-library/react';
import '@testing-library/jest-dom';
import {MemoryRouter} from 'react-router';
import {of} from 'rxjs';

import {Provider} from '../../../shared/context';
import {services} from '../../../shared/services';
import {ClustersList, compareServerVersion} from './clusters-list';

jest.mock('../../../shared/components', () => ({
    ...jest.requireActual('../../../shared/components'),
    Page: (props: {children?: React.ReactNode}) => props.children,
    FlexTopBar: (): null => null,
    Paginate: (props: {data: any[]; children: (data: any[]) => React.ReactNode}) => props.children(props.data),
    ActionMenu: (props: {items: {title: string; action: () => void}[]}) =>
        props.items.map(item => (
            <button key={item.title} onClick={item.action}>
                {item.title}
            </button>
        ))
}));

const mockSidebarTarget = {current: document.createElement('div')};
jest.mock('../../../sidebar/sidebar', () => ({useSidebarTarget: () => mockSidebarTarget}));

jest.mock('../../../shared/services', () => ({
    services: {
        clusters: {list: jest.fn(), delete: jest.fn(() => Promise.resolve())},
        viewPreferences: {getPreferences: () => of({hideSidebar: false})}
    }
}));

const sign = (n: number) => (n < 0 ? -1 : n > 0 ? 1 : 0);

test('compares equal versions as equal', () => {
    expect(compareServerVersion('v1.28.3', 'v1.28.3')).toEqual(0);
    expect(compareServerVersion('1.28.3', 'v1.28.3')).toEqual(0);
});

test('orders by major, then minor, then patch', () => {
    expect(sign(compareServerVersion('v1.28.3', 'v2.0.0'))).toEqual(-1);
    expect(sign(compareServerVersion('v1.28.3', 'v1.29.0'))).toEqual(-1);
    expect(sign(compareServerVersion('v1.28.3', 'v1.28.4'))).toEqual(-1);
    expect(sign(compareServerVersion('v2.0.0', 'v1.28.3'))).toEqual(1);
});

test('compares minor/patch numerically, not lexicographically', () => {
    expect(sign(compareServerVersion('v1.9.0', 'v1.10.0'))).toEqual(-1);
    expect(sign(compareServerVersion('v1.28.9', 'v1.28.10'))).toEqual(-1);
});

test('is case-insensitive on the v prefix', () => {
    expect(compareServerVersion('V1.28.3', 'v1.28.3')).toEqual(0);
});

test('treats empty versions as lowest', () => {
    expect(sign(compareServerVersion('', 'v1.0.0'))).toEqual(-1);
    expect(sign(compareServerVersion('v1.0.0', ''))).toEqual(1);
    expect(compareServerVersion('', '')).toEqual(0);
});

test('treats missing trailing components as zero', () => {
    expect(compareServerVersion('v1.28', 'v1.28.0')).toEqual(0);
    expect(sign(compareServerVersion('v1.28', 'v1.28.1'))).toEqual(-1);
});

test('ignores build metadata after a plus', () => {
    expect(compareServerVersion('v1.28.3+k3s1', 'v1.28.3')).toEqual(0);
    expect(compareServerVersion('v1.28.3+k3s1', 'v1.28.3+gke.99')).toEqual(0);
});

test('ranks a pre-release suffix below the plain release', () => {
    expect(sign(compareServerVersion('v1.28.3-gke.100', 'v1.28.3'))).toEqual(-1);
    expect(sign(compareServerVersion('v1.28.3', 'v1.28.3-rc.1'))).toEqual(1);
});

test('compares pre-release identifiers (numeric and non-numeric)', () => {
    expect(sign(compareServerVersion('v1.0.0-alpha', 'v1.0.0-beta'))).toEqual(-1);
    expect(sign(compareServerVersion('v1.0.0-alpha.1', 'v1.0.0-alpha.2'))).toEqual(-1);
    // numeric identifiers rank below alphanumeric ones
    expect(sign(compareServerVersion('v1.0.0-1', 'v1.0.0-alpha'))).toEqual(-1);
    // a larger set of fields outranks a smaller one when the prefix matches
    expect(sign(compareServerVersion('v1.0.0-alpha', 'v1.0.0-alpha.1'))).toEqual(-1);
});

test('sorts a list of versions ascending', () => {
    const versions = ['v1.10.0', 'v1.9.0', 'v2.0.1', 'v1.10.2', '', 'v1.9.10'];
    const sorted = [...versions].sort(compareServerVersion);
    expect(sorted).toEqual(['', 'v1.9.0', 'v1.9.10', 'v1.10.0', 'v1.10.2', 'v2.0.1']);
});

test('sorts pre-releases before their release in an ascending list', () => {
    const versions = ['v1.28.3', 'v1.28.3-rc.2', 'v1.28.3-rc.1', 'v1.28.2'];
    const sorted = [...versions].sort(compareServerVersion);
    expect(sorted).toEqual(['v1.28.2', 'v1.28.3-rc.1', 'v1.28.3-rc.2', 'v1.28.3']);
});

describe('ClustersList', () => {
    const cluster = {name: 'in-cluster', server: 'https://kubernetes.default.svc', info: {serverVersion: 'v1.33.0', connectionState: {status: 'Successful'}}};

    beforeEach(() => jest.clearAllMocks());

    it('refetches the clusters after deleting one', async () => {
        jest.mocked(services.clusters.list).mockResolvedValueOnce([cluster] as any).mockResolvedValueOnce([]);
        render(
            <QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>
                <Provider
                    value={{notifications: {show: jest.fn()}, navigation: {goto: jest.fn()}, popup: {confirm: () => Promise.resolve(true)}, history: {location: {search: ''}}} as any}>
                    <MemoryRouter>
                        <ClustersList />
                    </MemoryRouter>
                </Provider>
            </QueryClientProvider>
        );

        expect(screen.getByText('Loading...')).toBeInTheDocument();
        fireEvent.click(await screen.findByText('Delete'));

        await waitFor(() => expect(services.clusters.delete).toHaveBeenCalledWith('https://kubernetes.default.svc'));
        expect(await screen.findByText('No clusters connected')).toBeInTheDocument();
        expect(services.clusters.list).toHaveBeenCalledTimes(2);
    });
});
