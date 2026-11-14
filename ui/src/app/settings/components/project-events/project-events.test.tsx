import * as React from 'react';
import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {render, screen} from '@testing-library/react';
import '@testing-library/jest-dom';

import {services} from '../../../shared/services';
import {ProjectEvents} from './project-events';

jest.mock('../../../shared/components', () => ({
    EventsList: (props: {events: {message: string}[]}) => props.events.map(e => <p key={e.message}>{e.message}</p>)
}));

jest.mock('../../../shared/services', () => ({
    services: {projects: {events: jest.fn()}}
}));

describe('ProjectEvents', () => {
    it('renders the project events', async () => {
        jest.mocked(services.projects.events).mockResolvedValue([{message: 'synced'}] as any);
        render(
            <QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>
                <ProjectEvents projectName='my-proj' />
            </QueryClientProvider>
        );

        expect(await screen.findByText('synced')).toBeInTheDocument();
        expect(services.projects.events).toHaveBeenCalledWith('my-proj');
    });
});
