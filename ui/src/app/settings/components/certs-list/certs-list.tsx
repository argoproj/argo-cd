import {DropDownMenu, FormField, NotificationType, SlidingPanel} from 'argo-ui';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import {Form, FormApi, Text, TextArea} from 'react-form';
import {RouteComponentProps} from 'react-router';
import {t} from 'i18next';
import {Trans} from 'react-i18next';

import {DataLoader, EmptyState, ErrorNotification, Page} from '../../../shared/components';
import {AppContext} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import en from '../../../locales/en';

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
                title={t('certs-list.breadcrumbs.1', en['certs-list.breadcrumbs.1'])}
                toolbar={{
                    breadcrumbs: [
                        {title: t('certs-list.breadcrumbs.0', en['certs-list.breadcrumbs.0']), path: '/settings'},
                        {title: t('certs-list.breadcrumbs.1', en['certs-list.breadcrumbs.1'])}
                    ],
                    actionMenu: {
                        className: 'fa fa-plus',
                        items: [
                            {
                                title: t('certs-list.toolbar.add-tls-certificate', en['certs-list.toolbar.add-tls-certificate']),
                                iconClassName: 'fa fa-plus',
                                action: () => (this.showAddTLSCertificate = true)
                            },
                            {
                                title: t('certs-list.toolbar.add-ssh-known-hosts', en['certs-list.toolbar.add-ssh-known-hosts']),
                                iconClassName: 'fa fa-plus',
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
                                                <div className='columns small-3'>{t('certs-list.head.server-name', en['certs-list.head.server-name'])}</div>
                                                <div className='columns small-3'>{t('certs-list.head.cert-type', en['certs-list.head.cert-type'])}</div>
                                                <div className='columns small-6'>{t('certs-list.head.cert-info', en['certs-list.head.cert-info'])}</div>
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
                                                                    title: t('remove', en['remove']),
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
                                        <h4>{t('certs-list.empty.title', en['certs-list.empty.title'])}</h4>
                                        <h5>{t('certs-list.empty.description', en['certs-list.empty.description'])}</h5>
                                        <button className='argo-button argo-button--base' onClick={() => (this.showAddTLSCertificate = true)}>
                                            {t('certs-list.empty.add-tls-certificates', en['certs-list.empty.add-tls-certificates'])}
                                        </button>{' '}
                                        <button className='argo-button argo-button--base' onClick={() => (this.showAddSSHKnownHosts = true)}>
                                            {t('certs-list.empty.add-ssh-known-hosts', en['certs-list.empty.add-ssh-known-hosts'])}
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
                                {t('create', en['create'])}
                            </button>{' '}
                            <button onClick={() => (this.showAddTLSCertificate = false)} className='argo-button argo-button--base-o'>
                                {t('cancel', en['cancel'])}
                            </button>
                        </div>
                    }>
                    <Form
                        onSubmit={params => this.addTLSCertificate(params as NewTLSCertParams)}
                        getApi={api => (this.formApiTLS = api)}
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
                                    <p>{t('certs-list.add-tls-certificate.title', en['certs-list.add-tls-certificate.title'])}</p>
                                    <div className='argo-form-row'>
                                        <FormField
                                            formApi={formApiTLS}
                                            label={t('certs-list.add-tls-certificate.repository-server-name', en['certs-list.add-tls-certificate.repository-server-name'])}
                                            field='serverName'
                                            component={Text}
                                        />
                                    </div>
                                    <div className='argo-form-row'>
                                        <FormField
                                            formApi={formApiTLS}
                                            label={t('certs-list.add-tls-certificate.tls-certificate', en['certs-list.add-tls-certificate.tls-certificate'])}
                                            field='certData'
                                            component={TextArea}
                                        />
                                    </div>
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
                                {t('create', en['create'])}
                            </button>{' '}
                            <button onClick={() => (this.showAddSSHKnownHosts = false)} className='argo-button argo-button--base-o'>
                                {t('cancel', en['cancel'])}
                            </button>
                        </div>
                    }>
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
                                <div className='white-box'>
                                    <p>{t('certs-list.add-ssh-known-hosts.title', en['certs-list.add-ssh-known-hosts.title'])}</p>
                                    <p>
                                        <Trans
                                            i18nKey='certs-list.add-ssh-known-hosts.description'
                                            defaults={en['certs-list.add-ssh-known-hosts.description']}
                                            components={{code: <code />}}
                                        />
                                    </p>
                                    <p>
                                        <strong>{t('certs-list.add-ssh-known-hosts.notice', en['certs-list.add-ssh-known-hosts.notice'])}</strong>
                                    </p>
                                    <div className='argo-form-row'>
                                        <FormField
                                            formApi={formApiSSH}
                                            label={t('certs-list.add-ssh-known-hosts.ssh-known-hosts-data', en['certs-list.add-ssh-known-hosts.ssh-known-hosts-data'])}
                                            field='certData'
                                            component={TextArea}
                                        />
                                    </div>
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
                content: <ErrorNotification title={t('certs-list.add-tls-certificate.failed', en['certs-list.add-tls-certificate.failed'])} e={e} />,
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
                                throw new Error(t('certs-list.add-ssh-known-hosts.invalid-ssh-subtype', en['certs-list.add-ssh-known-hosts.invalid-ssh-subtype']) + subType);
                            }
                        }
                    }
                });
            if (knownHostEntries.length === 0) {
                throw new Error(t('certs-list.add-ssh-known-hosts.invalid-known-hosts-data', en['certs-list.add-ssh-known-hosts.invalid-known-hosts-data']));
            }
            await services.certs.create({items: knownHostEntries, metadata: null});
            this.showAddSSHKnownHosts = false;
            this.loader.reload();
        } catch (e) {
            this.appContext.apis.notifications.show({
                content: <ErrorNotification title={t('certs-list.add-ssh-known-hosts.failed', en['certs-list.add-ssh-known-hosts.failed'])} e={e} />,
                type: NotificationType.Error
            });
        }
    }

    private async removeCert(serverName: string, certType: string, certSubType: string) {
        const confirmed = await this.appContext.apis.popup.confirm(
            t('certs-list.remove-cert.popup.title', en['certs-list.remove-cert.popup.title']),
            t('certs-list.remove-cert.popup.description', en['certs-list.remove-cert.popup.description'], {certType, serverName})
        );
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
