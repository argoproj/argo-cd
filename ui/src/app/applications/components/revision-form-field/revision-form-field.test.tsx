import * as React from 'react';
import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {fireEvent, render, screen} from '@testing-library/react';
import '@testing-library/jest-dom';

import {newQueryClient} from '../../../shared/query-client';
import {services} from '../../../shared/services';
import {RevisionFormField} from './revision-form-field';

// FormField renders the suggestions it would offer, and DropDownMenu its items as buttons, so the tests check what the
// field is given without driving the autocomplete popup.
jest.mock('argo-ui', () => ({
    ...jest.requireActual('argo-ui'),
    FormField: (props: {componentProps: {items: string[]}}) => <div data-testid='suggestions'>{props.componentProps.items.join(',')}</div>,
    DropDownMenu: (props: {items: {title: string; action: () => any}[]}) => (
        <>
            {props.items.map(item => (
                <button key={item.title} onClick={item.action}>
                    {item.title}
                </button>
            ))}
        </>
    )
}));
jest.mock('../../../shared/services', () => ({services: {repos: {revisions: jest.fn(), ociTags: jest.fn()}}}));
jest.mock('../../../shared/components', () => ({RevisionHelpIcon: () => null}));

const formApi = {getFormState: () => ({values: {spec: {project: 'my-project'}}})} as any;
const refs = {branches: ['main', 'release-1.0'], tags: ['v1.0.0']};

const renderField = (props: Partial<React.ComponentProps<typeof RevisionFormField>> = {}, client = new QueryClient({defaultOptions: {queries: {retry: false}}})) =>
    render(
        <QueryClientProvider client={client}>
            <RevisionFormField formApi={formApi} repoURL='https://github.com/org/app' {...props} />
        </QueryClientProvider>
    );

describe('RevisionFormField', () => {
    beforeEach(() => jest.clearAllMocks());

    it('suggests branches, then tags without loading the refs again', async () => {
        jest.mocked(services.repos.revisions).mockResolvedValue(refs as any);
        renderField();

        expect(await screen.findByText('HEAD,main,release-1.0')).toBeInTheDocument();
        fireEvent.click(screen.getByText('Tags'));
        expect(screen.getByTestId('suggestions')).toHaveTextContent('HEAD,v1.0.0');
        fireEvent.click(screen.getByText('Branches'));
        expect(screen.getByTestId('suggestions')).toHaveTextContent('HEAD,main,release-1.0');

        expect(services.repos.revisions).toHaveBeenCalledTimes(1);
        expect(services.repos.revisions).toHaveBeenCalledWith('https://github.com/org/app', 'my-project');
        expect(services.repos.ociTags).not.toHaveBeenCalled();
    });

    it('suggests only tags when the field is limited to tags', async () => {
        jest.mocked(services.repos.revisions).mockResolvedValue(refs as any);
        renderField({revisionType: 'Tags'});

        expect(await screen.findByText('HEAD,v1.0.0')).toBeInTheDocument();
        expect(screen.queryByText('Branches')).not.toBeInTheDocument();
    });

    it('suggests OCI tags for an OCI repository', async () => {
        jest.mocked(services.repos.ociTags).mockResolvedValue({tags: ['1.2.3', '1.2.4']} as any);
        renderField({repoURL: 'oci://registry.example.com/charts/app', repoType: 'oci'});

        expect(await screen.findByText('HEAD,1.2.3,1.2.4')).toBeInTheDocument();
        expect(services.repos.ociTags).toHaveBeenCalledWith('oci://registry.example.com/charts/app', 'my-project');
        expect(services.repos.revisions).not.toHaveBeenCalled();
        expect(screen.queryByText('Tags')).not.toBeInTheDocument();
    });

    it('suggests only HEAD without an error notification when the refs fail to load', async () => {
        jest.mocked(services.repos.revisions).mockRejectedValue(new Error('forbidden'));
        const show = jest.fn();
        renderField({}, newQueryClient({show} as any));

        await screen.findByText('HEAD');
        await new Promise(resolve => setTimeout(resolve, 0));
        expect(screen.getByTestId('suggestions')).toHaveTextContent(/^HEAD$/);
        expect(show).not.toHaveBeenCalled();
    });

    it('loads nothing and suggests nothing without a repository', () => {
        renderField({repoURL: ''});

        expect(screen.getByTestId('suggestions')).toHaveTextContent(/^$/);
        expect(services.repos.revisions).not.toHaveBeenCalled();
        expect(services.repos.ociTags).not.toHaveBeenCalled();
    });

    it('loads the refs again when the repository changes', async () => {
        jest.mocked(services.repos.revisions).mockImplementation(async repo => (repo.endsWith('/other') ? {branches: ['dev'], tags: []} : refs) as any);
        const client = new QueryClient({defaultOptions: {queries: {retry: false}}});
        const {rerender} = renderField({}, client);
        expect(await screen.findByText('HEAD,main,release-1.0')).toBeInTheDocument();

        rerender(
            <QueryClientProvider client={client}>
                <RevisionFormField formApi={formApi} repoURL='https://github.com/org/other' />
            </QueryClientProvider>
        );
        expect(await screen.findByText('HEAD,dev')).toBeInTheDocument();
        expect(services.repos.revisions).toHaveBeenCalledTimes(2);
    });
});
