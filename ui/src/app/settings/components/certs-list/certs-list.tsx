import {DropDownMenu, FormField, NotificationType, SlidingPanel} from 'argo-ui';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import {Form, FormApi, Text, TextArea} from 'react-form';
import {RouteComponentProps} from 'react-router';

import {DataLoader, EmptyState, ErrorNotification, Page} from '../../../shared/components';
import {AppContext} from '../../../shared/context';
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

export class CertsList extends React.Component<RouteComponentProps<any>> {
    public static contextTypes = {
        router: PropTypes.object,
        apis: PropTypes.object,
        history: PropTypes.object
    };

    private formApiTLS: FormApi;
    private formApiSSH: FormApi;
    private loader: DataLoader;

    public render() {
        return (
            <Page
                title='Repository certificates'
                toolbar={{
                    breadcrumbs: [{title: 'Settings', path: '/settings'}, {title: 'Repository certificates'}],
                    actionMenu: {
                        className: 'fa fa-plus',
                        items: [
                            {
                                title: 'Add TLS certificate',
                                action: () => (this.showAddTLSCertificate = true)
                            },
                            {
                                title: 'Add SSH known hosts',
                                action: () => (this.showAddSSHKnownHosts = true)
                            }
                        ]
                    }
                }}>
                <div className='certs-list'>
                    <div className='argo-container'>
                        <DataLoader load={() => services.certs.list()} ref={loader => (this.loader = loader)}>
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
                                                                    action: () => this.removeCert(cert.serverName, cert.certType, cert.certSubType)
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
                                        <button className='argo-button argo-button--base' onClick={() => (this.showAddTLSCertificate = true)}>
                                            Add TLS certificates
                                        </button>{' '}
                                        <button className='argo-button argo-button--base' onClick={() => (this.showAddSSHKnownHosts = true)}>
                                            Add SSH known hosts
                                        </button>
                                    </EmptyState>
                                )
                            }
                        </DataLoader>
                    </div>
                </div>
                <SlidingPanel
                    isShown={this.showAddTLSCertificate}
                    onClose={() => (this.showAddTLSCertificate = false)}
                    header={
                        <div>
                            <button className='argo-button argo-button--base' onClick={() => this.formApiTLS.submitForm(null)}>
                                Create
                            </button>{' '}
                            <button onClick={() => (this.showAddTLSCertificate = false)} className='argo-button argo-button--base-o'>
                                Cancel
                            </button>
                        </div>
                    }>
                    <h4>Create TLS repository certificate</h4>
                    <Form
                        onSubmit={params => this.addTLSCertificate(params as NewTLSCertParams)}
                        getApi={api => (this.formApiTLS = api)}
                        preSubmit={(params: NewTLSCertParams) => ({
                            serverName: params.serverName,
                            certData: btoa(params.certData)
                        })}
                        validateError={(params: NewTLSCertParams) => ({
                            serverName: !params.serverName && 'Repository server name is required',
                            certData: !params.certData && 'Certificate data is required'
                        })}>
                        {formApiTLS => (
                            <form onSubmit={formApiTLS.submitForm} role='form' className='certs-list width-control' encType='multipart/form-data'>
                                <div className='argo-form-row'>
                                    <FormField formApi={formApiTLS} label='Repository server name' field='serverName' component={Text} />
                                </div>
                                <div className='argo-form-row'>
                                    <FormField formApi={formApiTLS} label='TLS certificate (PEM format)' field='certData' component={TextArea} />
                                </div>
                            </form>
                        )}
                    </Form>
                </SlidingPanel>
                <SlidingPanel
                    isShown={this.showAddSSHKnownHosts}
                    onClose={() => (this.showAddSSHKnownHosts = false)}
                    header={
                        <div>
                            <button className='argo-button argo-button--base' onClick={() => this.formApiSSH.submitForm(null)}>
                                Create
                            </button>{' '}
                            <button onClick={() => (this.showAddSSHKnownHosts = false)} className='argo-button argo-button--base-o'>
                                Cancel
                            </button>
                        </div>
                    }>
                    <h4>Create SSH known host entries</h4>
                    <p>
                        Paste SSH known hosts data in the text area below, one entry per line. You can use output from e.g. <code>ssh-keyscan</code> or the contents of an{' '}
                        <code>ssh_known_hosts</code> file in a verbatim way. Lines starting with <code>#</code> will be treated as comments and be ignored.
                    </p>
                    <p>
                        <strong>Make sure there are no linebreaks in the keys.</strong>
                    </p>
                    <Form
                        onSubmit={params => this.addSSHKnownHosts(params as NewSSHKnownHostParams)}
                        getApi={api => (this.formApiSSH = api)}
                        preSubmit={(params: NewSSHKnownHostParams) => ({
                            certData: btoa(params.certData)
                        })}
                        validateError={(params: NewSSHKnownHostParams) => ({
                            certData: !params.certData && 'SSH known hosts data is required'
                        })}>
                        {formApiSSH => (
                            <form onSubmit={formApiSSH.submitForm} role='form' className='certs-list width-control' encType='multipart/form-data'>
                                <div className='argo-form-row'>
                                    <FormField formApi={formApiSSH} label='SSH known hosts data' field='certData' component={TextArea} />
                                </div>
                            </form>
                        )}
                    </Form>
                </SlidingPanel>
            </Page>
        );
    }

    private clearForms() {
        this.formApiSSH.resetAll();
        this.formApiTLS.resetAll();
    }

    private async addTLSCertificate(params: NewTLSCertParams) {
        try {
            await services.certs.create({items: [{serverName: params.serverName, certType: 'https', certData: params.certData, certSubType: '', certInfo: ''}], metadata: null});
            this.showAddTLSCertificate = false;
            this.loader.reload();
        } catch (e) {
            this.appContext.apis.notifications.show({
                content: <ErrorNotification title='Unable to add TLS certificate' e={e} />,
                type: NotificationType.Error
            });
        }
    }

    private async addSSHKnownHosts(params: NewSSHKnownHostParams) {
        try {
            let knownHostEntries: models.RepoCert[] = [];
            atob(params.certData)
                .split('\n')
                .forEach(function processEntry(item, index) {
                    const trimmedLine = item.trimLeft();
                    if (trimmedLine.startsWith('#') === false) {
                        const knownHosts = trimmedLine.split(' ', 3);
                        if (knownHosts.length === 3) {
                            // Perform a little sanity check on the data - server
                            // checks too, but let's not send it invalid data in
                            // the first place.
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
            this.showAddSSHKnownHosts = false;
            this.loader.reload();
        } catch (e) {
            this.appContext.apis.notifications.show({
                content: <ErrorNotification title='Unable to add SSH known hosts data' e={e} />,
                type: NotificationType.Error
            });
        }
    }

    private async removeCert(serverName: string, certType: string, certSubType: string) {
        const confirmed = await this.appContext.apis.popup.confirm('Remove certificate', 'Are you sure you want to remove ' + certType + ' certificate for ' + serverName + '?');
        if (confirmed) {
            await services.certs.delete(serverName, certType, certSubType);
            this.loader.reload();
        }
    }

    private get showAddTLSCertificate() {
        return new URLSearchParams(this.props.location.search).get('addTLSCert') === 'true';
    }

    private set showAddTLSCertificate(val: boolean) {
        this.clearForms();
        this.appContext.router.history.push(`${this.props.match.url}?addTLSCert=${val}`);
    }

    private get showAddSSHKnownHosts() {
        return new URLSearchParams(this.props.location.search).get('addSSHKnownHosts') === 'true';
    }

    private set showAddSSHKnownHosts(val: boolean) {
        this.clearForms();
        this.appContext.router.history.push(`${this.props.match.url}?addSSHKnownHosts=${val}`);
    }

    private get appContext(): AppContext {
        return this.context as AppContext;
    }
}
