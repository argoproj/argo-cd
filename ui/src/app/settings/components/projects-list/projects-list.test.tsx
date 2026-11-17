import * as React from 'react';
import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {fireEvent, render, screen} from '@testing-library/react';
import '@testing-library/jest-dom';
import {MemoryRouter} from 'react-router';

import {Provider} from '../../../shared/context';
import {services} from '../../../shared/services';
import {ProjectsList} from './projects-list';

jest.mock('../../../shared/components', () => ({
    ...jest.requireActual('../../../shared/components'),
    Page: (props: {children?: React.ReactNode}) => props.children,
    FlexTopBar: (): null => null,
    Paginate: (props: {data: any[]; children: (data: any[]) => React.ReactNode}) => props.children(props.data)
}));

jest.mock('../../../shared/services', () => ({
    services: {projects: {list: jest.fn()}}
}));

const renderList = () =>
    render(
        <QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>
            <Provider value={{notifications: {show: jest.fn()}, navigation: {goto: jest.fn()}, popup: {}, history: {location: {search: ''}}} as any}>
                <MemoryRouter>
                    <ProjectsList />
                </MemoryRouter>
            </Provider>
        </QueryClientProvider>
    );

describe('ProjectsList', () => {
    beforeEach(() => jest.clearAllMocks());

    it('renders the loaded projects', async () => {
        jest.mocked(services.projects.list).mockResolvedValue([{metadata: {name: 'my-proj'}, spec: {description: 'my description'}}] as any);
        renderList();

        expect(screen.getByText('Loading...')).toBeInTheDocument();
        expect(await screen.findByText('my-proj')).toBeInTheDocument();
        expect(screen.getByText('my description')).toBeInTheDocument();
    });

    it('offers a retry when the projects fail to load', async () => {
        jest.mocked(services.projects.list).mockRejectedValueOnce(new Error('boom')).mockResolvedValueOnce([]);
        renderList();

        fireEvent.click(await screen.findByText('try again'));

        expect(await screen.findByText('No projects yet')).toBeInTheDocument();
        expect(services.projects.list).toHaveBeenCalledTimes(2);
    });
});
