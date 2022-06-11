import * as React from 'react';

import {DataLoader, EmptyState, Page} from '../../../shared/components';
import {Context} from '../../../shared/context';
import {services} from '../../../shared/services';

export const AccountsList = () => {
    const ctx = React.useContext(Context);
    return (
        <Page
            title='Accounts'
            toolbar={{
                breadcrumbs: [{title: 'Settings', path: '/settings'}, {title: 'Accounts'}]
            }}>
            <div className='argo-container'>
                <DataLoader load={() => services.accounts.list()}>
                    {accounts =>
                        (accounts.length > 0 && (
                            <div className='argo-table-list argo-table-list--clickable'>
                                <div className='argo-table-list__head'>
                                    <div className='row'>
                                        <div className='columns small-3'>NAME</div>
                                        <div className='columns small-3'>ENABLED</div>
                                        <div className='columns small-6'>CAPABILITIES</div>
                                    </div>
                                </div>
                                {accounts.map(account => (
                                    <div className='argo-table-list__row' key={account.name} onClick={() => ctx.navigation.goto(`./${account.name}`)}>
                                        <div className='row'>
                                            <div className='columns small-3'>{account.name}</div>
                                            <div className='columns small-3'>{(account.enabled && 'true') || 'false'}</div>
                                            <div className='columns small-6'>{account.capabilities && account.capabilities.join(', ')}</div>
                                        </div>
                                    </div>
                                ))}
                            </div>
                        )) || (
                            <EmptyState icon='fa fa-user-circle'>
                                <h4>No accounts yet</h4>
                                <h5>Define new accounts in Argo CD configuration</h5>
                            </EmptyState>
                        )
                    }
                </DataLoader>
            </div>
        </Page>
    );
};
