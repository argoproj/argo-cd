import * as React from 'react';
import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {fireEvent, render, screen} from '@testing-library/react';
import '@testing-library/jest-dom';
import {MemoryRouter, Route} from 'react-router';

import {Provider} from '../../../shared/context';
import {DetailedProjectsResponse} from '../../../shared/models';
import {services} from '../../../shared/services';
import {ProjectDetails} from './project-details';

jest.mock('../../../shared/components', () => ({
    ...jest.requireActual('../../../shared/components'),
    Page: (props: {children?: React.ReactNode}) => props.children,
    BadgePanel: () => null
}));

jest.mock('../../../shared/components/deep-links', () => ({DeepLinks: (): null => null}));

jest.mock('../project-events/project-events', () => ({ProjectEvents: (): null => null}));

jest.mock('../../../shared/services', () => ({
    services: {
        projects: {
            getDetailed: jest.fn(),
            getLinks: jest.fn(() => Promise.resolve({items: []})),
            getSyncWindows: jest.fn(() => Promise.resolve({windows: []}))
        },
        applications: {
            list: jest.fn(() => Promise.resolve({items: [{}, {}, {}]}))
        }
    }
}));

const detailed = {
    project: {metadata: {name: 'my-proj'}, spec: {description: 'my description'}, status: {}},
    globalProjects: [],
    repositories: [],
    clusters: []
} as DetailedProjectsResponse;

const renderDetails = () =>
    render(
        <QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>
            <Provider value={{notifications: {show: jest.fn()}, navigation: {goto: jest.fn()}, popup: {}} as any}>
                <MemoryRouter initialEntries={['/settings/projects/my-proj']}>
                    <Route path='/settings/projects/:name' component={ProjectDetails} />
                </MemoryRouter>
            </Provider>
        </QueryClientProvider>
    );

describe('ProjectDetails', () => {
    beforeEach(() => jest.clearAllMocks());

    it('renders the project and its application count once loaded', async () => {
        jest.mocked(services.projects.getDetailed).mockResolvedValue(detailed);
        renderDetails();

        expect(screen.getByText('Loading...')).toBeInTheDocument();
        expect(await screen.findByText('my description')).toBeInTheDocument();
        expect(await screen.findByRole('link', {name: '3'})).toHaveAttribute('href', '/applications?proj=my-proj');
        expect(services.projects.getDetailed).toHaveBeenCalledWith('my-proj');
        expect(services.applications.list).toHaveBeenCalledWith(['my-proj'], 'application');
    });

    it('offers a retry when the project fails to load', async () => {
        jest.mocked(services.projects.getDetailed).mockRejectedValueOnce(new Error('boom')).mockResolvedValueOnce(detailed);
        renderDetails();

        fireEvent.click(await screen.findByText('try again'));

        expect(await screen.findByText('my description')).toBeInTheDocument();
        expect(services.projects.getDetailed).toHaveBeenCalledTimes(2);
    });
});
