import {DropDownMenu, FormField, NotificationType, SlidingPanel} from 'argo-ui';
import React, {useRef, useContext} from 'react';
import {Form, FormApi, Text, TextArea} from 'react-form';
import {withRouter, RouteComponentProps} from 'react-router-dom';

import {DataLoader, EmptyState, ErrorNotification, Page} from '../../../shared/components';
import {Context} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';

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
    const ctx = useContext(Context);

    const formApiTLS = useRef<FormApi | null>(null);
    const formApiSSH = useRef<FormApi | null>(null);
    const loader = useRef<DataLoader | null>(null);

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
        <Page
            title='Repository certificates and known hosts'
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
                }
            }}>
            <div className='certs-list'>
                <div className='argo-container'>
                    <DataLoader load={() => services.certs.list()} ref={ref => (loader.current = ref)}>
                        {(certs: models.RepoCert[]) =>
                            (certs.length > 0 && (
                                <div className='argo-table-list'>
                                    <div className='argo-table-list__head'>
                                        <div className='row'>
                                            <div className='columns small-3'>SERVER NAME</div>
                                            <div className='columns small-3'>CERT TYPE</div>
                                            <div className='columns small-6'>CERT INFO</div>
                                        </div>
                                    </div>
                                    {certs.map(cert => (
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
                                                            <button className='argo-button argo-button--light argo-button--lg argo-button--short'>
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
                            )) || (
                                <EmptyState icon='argo-icon-git'>
                                    <h4>No certificates configured</h4>
                                    <h5>You can add further certificates below..</h5>
                                    <button className='argo-button argo-button--base' onClick={() => setAddTLSCertificate(true)}>
                                        Add TLS certificates
                                    </button>{' '}
                                    <button className='argo-button argo-button--base' onClick={() => setAddSSHKnownHosts(true)}>
                                        Add SSH known hosts
                                    </button>
                                </EmptyState>
                            )
                        }
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
