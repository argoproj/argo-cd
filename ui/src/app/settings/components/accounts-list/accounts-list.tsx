import * as React from 'react';
import * as ReactDOM from 'react-dom';

import {DataLoader, EmptyState, Page, Paginate, SearchBar} from '../../../shared/components';
import {Context} from '../../../shared/context';
import {services} from '../../../shared/services';
import {useQuery} from '../../../shared/hooks/query';
import {FlexTopBar} from '../../../shared/components';
import {useSidebarTarget} from '../../../sidebar/sidebar';
import {AccountsFilter, AccountsListPreferences, getAccountFilterResults, filterAccounts} from './accounts-filter';

export const AccountsList = () => {
    const ctx = React.useContext(Context);
    const query = useQuery();
    const searchText = query.get('search') || '';
    const [page, setPage] = React.useState(0);
    const sidebarTarget = useSidebarTarget();

    const [filterPref, setFilterPref] = React.useState<AccountsListPreferences>({
        statusFilter: query.getAll('status') || [],
        capabilitiesFilter: query.getAll('capabilities') || []
    });

    const updateFilterPref = (newPref: AccountsListPreferences) => {
        setFilterPref(newPref);
        ctx.navigation.goto(
            '.',
            {
                status: newPref.statusFilter.length > 0 ? newPref.statusFilter : null,
                capabilities: newPref.capabilitiesFilter.length > 0 ? newPref.capabilitiesFilter : null,
                search: searchText || null
            },
            {replace: true}
        );
        setPage(0);
    };

    return (
        <Page title='Accounts' toolbar={{breadcrumbs: [{title: 'Settings', path: '/settings'}, {title: 'Accounts'}]}}>
            <FlexTopBar
                toolbar={{
                    breadcrumbs: [{title: 'Settings', path: '/settings'}, {title: 'Accounts'}],
                    tools: (
                        <SearchBar
                            value={searchText}
                            onChange={value => {
                                ctx.navigation.goto(
                                    '.',
                                    {
                                        status: filterPref.statusFilter.length > 0 ? filterPref.statusFilter : null,
                                        capabilities: filterPref.capabilitiesFilter.length > 0 ? filterPref.capabilitiesFilter : null,
                                        search: value || null
                                    },
                                    {replace: true}
                                );
                                setPage(0);
                            }}
                            placeholder='Search accounts...'
                        />
                    )
                }}
            />
            <div className='argo-container'>
                <DataLoader load={() => services.accounts.list()}>
                    {accounts => {
                        const accountsWithFilter = getAccountFilterResults(accounts, filterPref);
                        const filteredByFilter = filterAccounts(accountsWithFilter);

                        const filteredAccounts = filteredByFilter.filter(
                            account =>
                                searchText === '' ||
                                account.name.toLowerCase().includes(searchText.toLowerCase()) ||
                                (account.capabilities && account.capabilities.join(', ').toLowerCase().includes(searchText.toLowerCase()))
                        );

                        return (
                            <>
                                {sidebarTarget &&
                                    ReactDOM.createPortal(<AccountsFilter accounts={accountsWithFilter} pref={filterPref} onChange={updateFilterPref} />, sidebarTarget.current)}
                                {filteredAccounts.length > 0 ? (
                                    <Paginate page={page} data={filteredAccounts} onPageChange={setPage} preferencesKey='accounts-list'>
                                        {accountsToDisplay => (
                                            <div className='argo-table-list argo-table-list--clickable'>
                                                <div className='argo-table-list__head'>
                                                    <div className='row'>
                                                        <div className='columns small-3'>NAME</div>
                                                        <div className='columns small-3'>ENABLED</div>
                                                        <div className='columns small-6'>CAPABILITIES</div>
                                                    </div>
                                                </div>
                                                {accountsToDisplay.map(account => (
                                                    <div className='argo-table-list__row' key={account.name} onClick={() => ctx.navigation.goto(`./${account.name}`)}>
                                                        <div className='row'>
                                                            <div className='columns small-3'>{account.name}</div>
                                                            <div className='columns small-3'>{(account.enabled && 'true') || 'false'}</div>
                                                            <div className='columns small-6'>{(account.capabilities || []).join(', ')}</div>
                                                        </div>
                                                    </div>
                                                ))}
                                            </div>
                                        )}
                                    </Paginate>
                                ) : accounts.length === 0 ? (
                                    <EmptyState icon='fa fa-user-circle'>
                                        <h4>No accounts yet</h4>
                                        <h5>Define new accounts in Argo CD configuration</h5>
                                    </EmptyState>
                                ) : (
                                    <EmptyState icon='fa fa-user-circle'>
                                        <h4>No accounts matched your search</h4>
                                        <h5>Try adjusting your search query</h5>
                                    </EmptyState>
                                )}
                            </>
                        );
                    }}
                </DataLoader>
            </div>
        </Page>
    );
};
