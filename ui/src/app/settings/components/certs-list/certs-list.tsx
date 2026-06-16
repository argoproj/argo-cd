import {DropDownMenu, FormField, NotificationType, SlidingPanel} from 'argo-ui';
import * as React from 'react';
import * as ReactDOM from 'react-dom';
import {Form, FormApi, Text, TextArea} from 'argo-ui';
import {withRouter, RouteComponentProps} from 'react-router-dom';

import {DataLoader, EmptyState, ErrorNotification, Page, Paginate, SearchBar} from '../../../shared/components';
import {Context} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {useQuery} from '../../../shared/hooks/query';
import {useListSort} from '../../../shared/hooks/use-list-sort';
import {FlexTopBar} from '../../../shared/components';
import {useSidebarTarget} from '../../../sidebar/sidebar';
import {CertsFilter, CertsListPreferences, getCertFilterResults, filterCerts} from './certs-filter';

require('./certs-list.scss');

interface NewTLSCertParams {
    serverName: string;
    certType: string;
    certData: string;
}

interface NewSSHKnownHostParams {
    certData: string;
}

export const CertsList = ({match, location}: RouteComponentProps) => {
    const ctx = React.useContext(Context);
    const query = useQuery();
    const searchText = query.get('search') || '';
    const [page, setPage] = React.useState(0);
    const sidebarTarget = useSidebarTarget();

    const formApiTLS = React.useRef<FormApi | null>(null);
    const formApiSSH = React.useRef<FormApi | null>(null);
    const loader = React.useRef<DataLoader | null>(null);

    const [filterPref, setFilterPref] = React.useState<CertsListPreferences>({
        certTypeFilter: query.getAll('certType') || []
    });

    type SortKey = 'serverName' | 'certType' | 'certInfo';
    const {sortKey, requestSort, sortIcon, compareString} = useListSort<SortKey>('serverName');

    const updateFilterPref = (newPref: CertsListPreferences) => {
        setFilterPref(newPref);
        ctx.navigation.goto(
            '.',
            {
                certType: newPref.certTypeFilter.length > 0 ? newPref.certTypeFilter : null,
                search: searchText || null
            },
            {replace: true}
        );
        setPage(0);
    };

    const clearForms = () => {
        formApiSSH.current.resetAll();
        formApiTLS.current.resetAll();
    };

    const addTLSCertificate = async (params: NewTLSCertParams) => {
        try {
            await services.certs.create({items: [{serverName: params.serverName, certType: 'https', certData: params.certData, certSubType: '', certInfo: ''}], metadata: null});
            setAddTLSCertificate(false);
            loader.current.reload();
        } catch (e) {
            ctx.notifications.show({
                content: <ErrorNotification title='Unable to add TLS certificate' e={e} />,
                type: NotificationType.Error
            });
        }
    };

    const addSSHKnownHosts = async (params: NewSSHKnownHostParams) => {
        try {
            let knownHostEntries: models.RepoCert[] = [];
            atob(params.certData)
                .split('\n')
                .forEach(function processEntry(item) {
                    const trimmedLine = item.trimLeft();
                    if (trimmedLine.startsWith('#') === false) {
                        const knownHosts = trimmedLine.split(' ', 3);
                        if (knownHosts.length === 3) {
                            // Perform a little sanity check on the data - server
                            // checks too, but let's not send it invalid data in
                            // the first place.
                            // eslint-disable-next-line no-useless-escape
                            const subType = knownHosts[1].match(/^(ssh\-[a-z0-9]+|ecdsa-[a-z0-9\-]+)$/gi);
                            if (subType != null) {
                                // Key could be valid for multiple hosts
                                const hostnames = knownHosts[0].split(',');
                                for (const hostname of hostnames) {
                                    knownHostEntries = knownHostEntries.concat({
                                        serverName: hostname,
                                        certType: 'ssh',
                                        certSubType: knownHosts[1],
                                        certData: btoa(knownHosts[2]),
                                        certInfo: ''
                                    });
                                }
                            } else {
                                throw new Error('Invalid SSH subtype: ' + subType);
                            }
                        }
                    }
                });
            if (knownHostEntries.length === 0) {
                throw new Error('No valid known hosts data entered');
            }
            await services.certs.create({items: knownHostEntries, metadata: null});
            setAddSSHKnownHosts(false);
            loader.current.reload();
        } catch (e) {
            ctx.notifications.show({
                content: <ErrorNotification title='Unable to add SSH known hosts data' e={e} />,
                type: NotificationType.Error
            });
        }
    };

    const removeCert = async (serverName: string, certType: string, certSubType: string) => {
        const confirmed = await ctx.popup.confirm('Remove certificate', 'Are you sure you want to remove ' + certType + ' certificate for ' + serverName + '?');
        if (confirmed) {
            await services.certs.delete(serverName, certType, certSubType);
            loader.current.reload();
        }
    };

    const showAddTLSCertificate = () => {
        return new URLSearchParams(location.search).get('addTLSCert') === 'true';
    };

    const setAddTLSCertificate = (val: boolean) => {
        clearForms();
        ctx.history.push({
            pathname: match.url,
            search: `?addTLSCert=${val}`
        });
    };

    const showAddSSHKnownHosts = () => {
        return new URLSearchParams(location.search).get('addSSHKnownHosts') === 'true';
    };

    const setAddSSHKnownHosts = (val: boolean) => {
        clearForms();
        ctx.history.push(`${match.url}?addSSHKnownHosts=${val}`);
    };

    return (
        <Page title='Repository certificates and known hosts' toolbar={{breadcrumbs: [{title: 'Settings', path: '/settings'}, {title: 'Repository certificates and known hosts'}]}}>
            <FlexTopBar
                toolbar={{
                    breadcrumbs: [{title: 'Settings', path: '/settings'}, {title: 'Repository certificates and known hosts'}],
                    actionMenu: {
                        className: 'fa fa-plus',
                        items: [
                            {
                                title: 'Add TLS certificate',
                                iconClassName: 'fa fa-plus',
                                action: () => setAddTLSCertificate(true)
                            },
                            {
                                title: 'Add SSH known hosts',
                                iconClassName: 'fa fa-plus',
                                action: () => setAddSSHKnownHosts(true)
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
                                        certType: filterPref.certTypeFilter.length > 0 ? filterPref.certTypeFilter : null,
                                        search: value || null
                                    },
                                    {replace: true}
                                );
                                setPage(0);
                            }}
                            placeholder='Search certificates...'
                        />
                    )
                }}
            />
            <div className='certs-list'>
                <div className='argo-container'>
                    <DataLoader
                        load={() => services.certs.list()}
                        ref={ref => {
                            loader.current = ref;
                        }}>
                        {(certs: models.RepoCert[]) => {
                            const certsWithFilter = getCertFilterResults(certs, filterPref);
                            const filteredByFilter = filterCerts(certsWithFilter);

                            const filteredCerts = filteredByFilter
                                .filter(
                                    cert =>
                                        searchText === '' ||
                                        cert.serverName.toLowerCase().includes(searchText.toLowerCase()) ||
                                        cert.certType.toLowerCase().includes(searchText.toLowerCase()) ||
                                        cert.certSubType.toLowerCase().includes(searchText.toLowerCase()) ||
                                        cert.certInfo.toLowerCase().includes(searchText.toLowerCase())
                                )
                                .sort((a, b) => {
                                    switch (sortKey) {
                                        case 'serverName':
                                            return compareString(a.serverName, b.serverName);
                                        case 'certType':
                                            return compareString(`${a.certType} ${a.certSubType}`, `${b.certType} ${b.certSubType}`);
                                        case 'certInfo':
                                            return compareString(a.certInfo, b.certInfo);
                                        default:
                                            return 0;
                                    }
                                });

                            return (
                                <>
                                    {sidebarTarget &&
                                        ReactDOM.createPortal(<CertsFilter certs={certsWithFilter} pref={filterPref} onChange={updateFilterPref} />, sidebarTarget.current)}
                                    {filteredCerts.length > 0 ? (
                                        <Paginate page={page} data={filteredCerts} onPageChange={setPage} preferencesKey='certs-list'>
                                            {certsToDisplay => (
                                                <div className='argo-table-list'>
                                                    <div className='argo-table-list__head'>
                                                        <div className='row'>
                                                            <div className='columns small-3 sortable' onClick={() => requestSort('serverName')}>
                                                                SERVER NAME
                                                                {sortIcon('serverName')}
                                                            </div>
                                                            <div className='columns small-3 sortable' onClick={() => requestSort('certType')}>
                                                                CERT TYPE
                                                                {sortIcon('certType')}
                                                            </div>
                                                            <div className='columns small-6 sortable' onClick={() => requestSort('certInfo')}>
                                                                CERT INFO
                                                                {sortIcon('certInfo')}
                                                            </div>
                                                        </div>
                                                    </div>
                                                    {certsToDisplay.map(cert => (
                                                        <div className='argo-table-list__row' key={cert.certType + '_' + cert.certSubType + '_' + cert.serverName}>
                                                            <div className='row'>
                                                                <div className='columns small-3'>
                                                                    <i className='icon argo-icon-git' /> {cert.serverName}
                                                                </div>
                                                                <div className='columns small-3'>
                                                                    {cert.certType} {cert.certSubType}
                                                                </div>
                                                                <div className='columns small-6'>
                                                                    {cert.certInfo}
                                                                    <DropDownMenu
                                                                        anchor={() => (
                                                                            <button
                                                                                className='argo-button argo-button--light argo-button--lg argo-button--short'
                                                                                onMouseDown={() => document.body.click()}>
                                                                                <i className='fa fa-ellipsis-v' />
                                                                            </button>
                                                                        )}
                                                                        items={[
                                                                            {
                                                                                title: 'Remove',
                                                                                action: () => removeCert(cert.serverName, cert.certType, cert.certSubType)
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
                                    ) : certs.length === 0 ? (
                                        <EmptyState icon='argo-icon-git'>
                                            <h4>No certificates configured</h4>
                                            <h5>You can add further certificates below.</h5>
                                            <button className='argo-button argo-button--base' onClick={() => setAddTLSCertificate(true)}>
                                                Add TLS certificates
                                            </button>{' '}
                                            <button className='argo-button argo-button--base' onClick={() => setAddSSHKnownHosts(true)}>
                                                Add SSH known hosts
                                            </button>
                                        </EmptyState>
                                    ) : (
                                        <EmptyState icon='argo-icon-git'>
                                            <h4>No certificates matched your search</h4>
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
                isShown={showAddTLSCertificate()}
                onClose={() => setAddTLSCertificate(false)}
                header={
                    <div>
                        <button className='argo-button argo-button--base' onClick={() => formApiTLS.current.submitForm(null)}>
                            Create
                        </button>{' '}
                        <button onClick={() => setAddTLSCertificate(false)} className='argo-button argo-button--base-o'>
                            Cancel
                        </button>
                    </div>
                }>
                <Form
                    onSubmit={params => addTLSCertificate(params as NewTLSCertParams)}
                    getApi={api => (formApiTLS.current = api)}
                    preSubmit={(params: NewTLSCertParams) => ({
                        serverName: params.serverName,
                        certData: btoa(params.certData)
                    })}
                    validateError={(params: NewTLSCertParams) => ({
                        serverName: !params.serverName && 'Repository Server Name is required',
                        certData: !params.certData && 'TLS Certificate is required'
                    })}>
                    {formApiTLS => (
                        <form onSubmit={formApiTLS.submitForm} role='form' className='certs-list width-control' encType='multipart/form-data'>
                            <div className='white-box'>
                                <p>CREATE TLS REPOSITORY CERTIFICATE</p>
                                <div className='argo-form-row'>
                                    <FormField formApi={formApiTLS} label='Repository Server Name' field='serverName' component={Text} />
                                </div>
                                <div className='argo-form-row'>
                                    <FormField formApi={formApiTLS} label='TLS Certificate (PEM format)' field='certData' component={TextArea} />
                                </div>
                            </div>
                        </form>
                    )}
                </Form>
            </SlidingPanel>
            <SlidingPanel
                isShown={showAddSSHKnownHosts()}
                onClose={() => setAddSSHKnownHosts(false)}
                header={
                    <div>
                        <button className='argo-button argo-button--base' onClick={() => formApiSSH.current.submitForm(null)}>
                            Create
                        </button>{' '}
                        <button onClick={() => setAddSSHKnownHosts(false)} className='argo-button argo-button--base-o'>
                            Cancel
                        </button>
                    </div>
                }>
                <Form
                    onSubmit={params => addSSHKnownHosts(params as NewSSHKnownHostParams)}
                    getApi={api => (formApiSSH.current = api)}
                    preSubmit={(params: NewSSHKnownHostParams) => ({
                        certData: btoa(params.certData)
                    })}
                    validateError={(params: NewSSHKnownHostParams) => ({
                        certData: !params.certData && 'SSH known hosts data is required'
                    })}>
                    {formApiSSH => (
                        <form onSubmit={formApiSSH.submitForm} role='form' className='certs-list width-control' encType='multipart/form-data'>
                            <div className='white-box'>
                                <p>CREATE SSH KNOWN HOST ENTRIES</p>
                                <p>
                                    Paste SSH known hosts data in the text area below, one entry per line. You can use output from <code>ssh-keyscan</code> or the contents on
                                    <code>ssh_known_hosts</code> file verbatim. Lines starting with <code>#</code> will be treated as comments and ignored.
                                </p>
                                <p>
                                    <strong>Make sure there are no linebreaks in the keys.</strong>
                                </p>
                                <div className='argo-form-row'>
                                    <FormField formApi={formApiSSH} label='SSH known hosts data' field='certData' component={TextArea} />
                                </div>
                            </div>
                        </form>
                    )}
                </Form>
            </SlidingPanel>
        </Page>
    );
};

export default withRouter(CertsList);
