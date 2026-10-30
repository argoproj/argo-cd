import * as React from 'react';
import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {render, screen} from '@testing-library/react';
import '@testing-library/jest-dom';

import {services} from '../../../shared/services';
import {ProjectRolePoliciesEdit} from './project-role-policies-edit';

jest.mock('../../../shared/services', () => ({
    services: {applications: {list: jest.fn(() => Promise.resolve({items: []}))}}
}));

describe('ProjectRolePoliciesEdit', () => {
    it('loads only the application names of the project', async () => {
        render(
            <QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>
                <ProjectRolePoliciesEdit projName='my-proj' roleName='admin' policies={[]} formApi={{} as any} newRole={false} />
            </QueryClientProvider>
        );

        expect(screen.getByText('Loading...')).toBeInTheDocument();
        expect(await screen.findByText('POLICY RULES')).toBeInTheDocument();
        expect(services.applications.list).toHaveBeenCalledWith(['my-proj'], 'application', {fields: ['items.metadata.name']});
    });
});
