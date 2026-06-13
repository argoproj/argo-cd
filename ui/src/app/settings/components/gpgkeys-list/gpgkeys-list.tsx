import {DropDownMenu, FormField, NotificationType, SlidingPanel} from 'argo-ui';
import * as React from 'react';
import * as ReactDOM from 'react-dom';
import {Form, FormApi, TextArea} from 'argo-ui';
import {withRouter, RouteComponentProps} from 'react-router-dom';

import {DataLoader, EmptyState, ErrorNotification, Page, Paginate, SearchBar} from '../../../shared/components';
import {Context} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {useQuery} from '../../../shared/hooks/query';
import {FlexTopBar} from '../../../shared/components';
import {useSidebarTarget} from '../../../sidebar/sidebar';
import {GpgKeysFilter, GpgKeysListPreferences, getGpgKeyFilterResults, filterGpgKeys} from './gpgkeys-filter';

require('./gpgkeys-list.scss');

interface NewGnuPGPublicKeyParams {
    keyData: string;
}

export const GpgKeysList = ({match, location}: RouteComponentProps) => {
    const ctx = React.useContext(Context);
    const query = useQuery();
    const searchText = query.get('search') || '';
    const [page, setPage] = React.useState(0);
    const sidebarTarget = useSidebarTarget();

    const formApi = React.useRef<FormApi | null>(null);
    const loader = React.useRef<DataLoader | null>(null);

    const [filterPref, setFilterPref] = React.useState<GpgKeysListPreferences>({
        keyTypeFilter: query.getAll('keyType') || []
    });

    const updateFilterPref = (newPref: GpgKeysListPreferences) => {
        setFilterPref(newPref);
        ctx.navigation.goto(
            '.',
            {
                keyType: newPref.keyTypeFilter.length > 0 ? newPref.keyTypeFilter : null,
                search: searchText || null
            },
            {replace: true}
        );
        setPage(0);
    };

    const clearForms = () => {
        formApi.current?.resetAll();
    };

    const validateKeyInputfield = (data: string): boolean => {
        if (data == null || data === '') {
            return false;
        }
        const str = data.trim();
        const startNeedle = '-----BEGIN PGP PUBLIC KEY BLOCK-----\n';
        const endNeedle = '\n-----END PGP PUBLIC KEY BLOCK-----';

        if (str.length < startNeedle.length + endNeedle.length) {
            return false;
        }
        if (!str.startsWith(startNeedle)) {
            return false;
        }
        if (!str.endsWith(endNeedle)) {
            return false;
        }
        return true;
    };

    const addGnuPGPublicKey = async (params: NewGnuPGPublicKeyParams) => {
        try {
            if (!validateKeyInputfield(params.keyData)) {
                throw {
                    name: 'Invalid key exception',
                    message: 'Invalid GnuPG key data found - must be ASCII armored'
                };
            }
            await services.gpgkeys.create({keyData: params.keyData});
            setAddGnuPGKey(false);
            loader.current?.reload();
        } catch (e) {
            ctx.notifications.show({
                content: <ErrorNotification title='Unable to add GnuPG public key' e={e} />,
                type: NotificationType.Error
            });
        }
    };

    const removeKey = async (keyId: string) => {
        const confirmed = await ctx.popup.confirm('Remove GPG public key', 'Are you sure you want to remove GPG key with ID ' + keyId + '?');
        if (confirmed) {
            await services.gpgkeys.delete(keyId);
            loader.current?.reload();
        }
    };

    const showAddGnuPGKey = () => {
        return new URLSearchParams(location.search).get('addGnuPGPublicKey') === 'true';
    };

    const setAddGnuPGKey = (val: boolean) => {
        clearForms();
        ctx.history.push(`${match.url}?addGnuPGPublicKey=${val}`);
    };

    return (
        <Page title='GnuPG public keys' toolbar={{breadcrumbs: [{title: 'Settings', path: '/settings'}, {title: 'GnuPG public keys'}]}}>
            <FlexTopBar
                toolbar={{
                    breadcrumbs: [{title: 'Settings', path: '/settings'}, {title: 'GnuPG public keys'}],
                    actionMenu: {
                        className: 'fa fa-plus',
                        items: [
                            {
                                title: 'Add GnuPG key',
                                iconClassName: 'fa fa-plus',
                                action: () => setAddGnuPGKey(true)
                            }
                        ]
                    },
                    tools: (
                        <SearchBar
                            value={searchText}
                            onChange={value => {
                                ctx.navigation.goto(
                                    '.',
                                    {
                                        keyType: filterPref.keyTypeFilter.length > 0 ? filterPref.keyTypeFilter : null,
                                        search: value || null
                                    },
                                    {replace: true}
                                );
                                setPage(0);
                            }}
                            placeholder='Search GPG keys...'
                        />
                    )
                }}
            />
            <div className='gpgkeys-list'>
                <div className='argo-container'>
                    <DataLoader
                        load={() => services.gpgkeys.list()}
                        ref={ref => {
                            loader.current = ref;
                        }}>
                        {(gpgkeys: models.GnuPGPublicKey[]) => {
                            const gpgkeysWithFilter = getGpgKeyFilterResults(gpgkeys, filterPref);
                            const filteredByFilter = filterGpgKeys(gpgkeysWithFilter);

                            const filteredGpgKeys = filteredByFilter.filter(
                                gpgkey =>
                                    searchText === '' ||
                                    gpgkey.keyID.toLowerCase().includes(searchText.toLowerCase()) ||
                                    gpgkey.owner.toLowerCase().includes(searchText.toLowerCase()) ||
                                    gpgkey.subType.toLowerCase().includes(searchText.toLowerCase())
                            );

                            return (
                                <>
                                    {sidebarTarget &&
                                        ReactDOM.createPortal(<GpgKeysFilter gpgkeys={gpgkeysWithFilter} pref={filterPref} onChange={updateFilterPref} />, sidebarTarget.current)}
                                    {filteredGpgKeys.length > 0 ? (
                                        <Paginate page={page} data={filteredGpgKeys} onPageChange={setPage} preferencesKey='gpgkeys-list'>
                                            {gpgkeysToDisplay => (
                                                <div className='argo-table-list'>
                                                    <div className='argo-table-list__head'>
                                                        <div className='row'>
                                                            <div className='columns small-3'>KEY ID</div>
                                                            <div className='columns small-3'>KEY TYPE</div>
                                                            <div className='columns small-6'>IDENTITY</div>
                                                        </div>
                                                    </div>
                                                    {gpgkeysToDisplay.map(gpgkey => (
                                                        <div className='argo-table-list__row' key={gpgkey.keyID}>
                                                            <div className='row'>
                                                                <div className='columns small-3'>
                                                                    <i className='fa fa-key' /> {gpgkey.keyID}
                                                                </div>
                                                                <div className='columns small-3'>{gpgkey.subType.toUpperCase()}</div>
                                                                <div className='columns small-6'>
                                                                    {gpgkey.owner}
                                                                    <DropDownMenu
                                                                        anchor={() => (
                                                                            <button className='argo-button argo-button--light argo-button--lg argo-button--short'>
                                                                                <i className='fa fa-ellipsis-v' />
                                                                            </button>
                                                                        )}
                                                                        items={[
                                                                            {
                                                                                title: 'Remove',
                                                                                action: () => removeKey(gpgkey.keyID)
                                                                            }
                                                                        ]}
                                                                    />
                                                                </div>
                                                            </div>
                                                        </div>
                                                    ))}
                                                </div>
                                            )}
                                        </Paginate>
                                    ) : gpgkeys.length === 0 ? (
                                        <EmptyState icon='fa fa-key'>
                                            <h4>No GnuPG public keys currently configured</h4>
                                            <h5>You can add GnuPG public keys below.</h5>
                                            <button className='argo-button argo-button--base' onClick={() => setAddGnuPGKey(true)}>
                                                Add GnuPG public key
                                            </button>
                                        </EmptyState>
                                    ) : (
                                        <EmptyState icon='fa fa-key'>
                                            <h4>No GPG keys matched your search</h4>
                                            <h5>Try adjusting your search query</h5>
                                        </EmptyState>
                                    )}
                                </>
                            );
                        }}
                    </DataLoader>
                </div>
            </div>
            <SlidingPanel
                isShown={showAddGnuPGKey()}
                onClose={() => setAddGnuPGKey(false)}
                header={
                    <div>
                        <button className='argo-button argo-button--base' onClick={() => formApi.current.submitForm(null)}>
                            Create
                        </button>{' '}
                        <button onClick={() => setAddGnuPGKey(false)} className='argo-button argo-button--base-o'>
                            Cancel
                        </button>
                    </div>
                }>
                <Form
                    onSubmit={params => addGnuPGPublicKey({keyData: params.keyData})}
                    getApi={api => (formApi.current = api)}
                    preSubmit={(params: NewGnuPGPublicKeyParams) => ({
                        keyData: params.keyData
                    })}
                    validateError={(params: NewGnuPGPublicKeyParams) => ({
                        keyData: !params.keyData && 'GnuPG public key data is required'
                    })}>
                    {formApi => (
                        <form onSubmit={formApi.submitForm} role='form' className='gpgkeys-list width-control' encType='multipart/form-data'>
                            <div className='white-box'>
                                <p>ADD GnuPG PUBLIC KEY</p>
                                <div className='argo-form-row'>
                                    <FormField formApi={formApi} label='GnuPG public key data (ASCII-armored)' field='keyData' component={TextArea} />
                                </div>
                            </div>
                        </form>
                    )}
                </Form>
            </SlidingPanel>
        </Page>
    );
};

export default withRouter(GpgKeysList);
