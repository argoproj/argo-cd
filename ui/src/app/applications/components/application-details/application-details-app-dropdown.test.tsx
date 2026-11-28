import * as React from 'react';
import {fireEvent, render, screen} from '@testing-library/react';
import {Context} from '../../../shared/context';
import * as models from '../../../shared/models';
import {ApplicationsDetailsAppDropdown} from './application-details-app-dropdown';

jest.mock('../../../shared/services', () => ({
    services: {
        applications: {
            list: jest.fn(() =>
                Promise.resolve({
                    items: [
                        {kind: 'Application', metadata: {name: 'guestbook', namespace: 'argocd'}},
                        {kind: 'Application', metadata: {name: 'guestbook-2', namespace: 'argocd'}},
						{kind: 'Application', metadata: {name: 'guestbook-set-a', namespace: 'argocd'}},
                        {kind: 'Application', metadata: {name: 'guestbook-set-b', namespace: 'argocd'}}
                    ]
                })
            ),
            getApplicationSet: jest.fn(() =>
                Promise.resolve({
                    status: {
                        resources: [
                            {kind: 'Application', name: 'guestbook-set-a', namespace: 'argocd'},
                            {kind: 'Application', name: 'guestbook-set-b', namespace: 'argocd'}
                        ]
                    }
                })
            )
        }
    }
}));

const ctx = {baseHref: '/', navigation: {goto: jest.fn()}} as any;

const renderDropdown = (props: {appName: string; objectListKind: string; application?: models.Application}) =>
    render(
        <Context.Provider value={ctx}>
            <ApplicationsDetailsAppDropdown {...props} />
        </Context.Provider>
    );

describe('ApplicationsDetailsAppDropdown', () => {
    beforeEach(() => jest.clearAllMocks());

    it('renders each application as a link to its details page', async () => {
        renderDropdown({appName: 'guestbook', objectListKind: 'application'});
        fireEvent.click(screen.getByText('guestbook')); 
        const link = await screen.findByRole('link', {name: /guestbook-2/});
        expect(link).toHaveAttribute('href', '/applications/argocd/guestbook-2');
    });

	it('navigates in the same tab on a plain click', async () => {
    	renderDropdown({appName: 'guestbook', objectListKind: 'application'});
        fireEvent.click(screen.getByText('guestbook'));        
        const link = await screen.findByRole('link', {name: /guestbook-2/});
        fireEvent.click(link);
        expect(ctx.navigation.goto).toHaveBeenCalled();
    });
    it('leaves modifier clicks to the browser', async () => {
      	renderDropdown({appName: 'guestbook', objectListKind: 'application'});
        fireEvent.click(screen.getByText('guestbook'));        
        const link = await screen.findByRole('link', {name: /guestbook-2/});
       	fireEvent.click(link, {ctrlKey: true});
       	expect(ctx.navigation.goto).not.toHaveBeenCalled();
    });
    it('renders ApplicationSet siblings as links', async () => {
        const application = {
            metadata: {
                name: 'guestbook-set-a',
                namespace: 'argocd',
                ownerReferences: [{kind: 'ApplicationSet', name: 'guestbook-set'}]
            }
        } as unknown as models.Application;

        renderDropdown({appName: 'guestbook-set-a', objectListKind: 'application', application});
        fireEvent.click(screen.getByText('guestbook-set-a'));
        await screen.findByText('IN SET: guestbook-set');
        const link = await screen.findByRole('link', {name: /guestbook-set-b/});
        expect(link).toHaveAttribute('href', '/applications/argocd/guestbook-set-b');
    });
});