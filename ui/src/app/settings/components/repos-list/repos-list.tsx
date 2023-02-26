import {AutocompleteField, DropDownMenu, FormField, FormSelect, HelpIcon, NotificationType, SlidingPanel, Tooltip} from 'argo-ui';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import {Form, FormValues, FormApi, Text, TextArea, FormErrors} from 'react-form';
import {RouteComponentProps} from 'react-router';
import {withTranslation} from 'react-i18next';

import {CheckboxField, ConnectionStateIcon, DataLoader, EmptyState, ErrorNotification, NumberField, Page, Repo, Spinner} from '../../../shared/components';
import {AppContext} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {RepoDetails} from '../repo-details/repo-details';
import en from '../../../locales/en';

require('./repos-list.scss');

interface NewSSHRepoParams {
    type: string;
    name: string;
    url: string;
    sshPrivateKey: string;
    insecure: boolean;
    enableLfs: boolean;
    proxy: string;
    project?: string;
}

export interface NewHTTPSRepoParams {
    type: string;
    name: string;
    url: string;
    username: string;
    password: string;
    tlsClientCertData: string;
    tlsClientCertKey: string;
    insecure: boolean;
    enableLfs: boolean;
    proxy: string;
    project?: string;
    forceHttpBasicAuth: boolean;
}

interface NewGitHubAppRepoParams {
    type: string;
    name: string;
    url: string;
    githubAppPrivateKey: string;
    githubAppId: bigint;
    githubAppInstallationId: bigint;
    githubAppEnterpriseBaseURL: string;
    tlsClientCertData: string;
    tlsClientCertKey: string;
    insecure: boolean;
    enableLfs: boolean;
    proxy: string;
    project?: string;
}

interface NewGoogleCloudSourceRepoParams {
    type: string;
    name: string;
    url: string;
    gcpServiceAccountKey: string;
    proxy: string;
    project?: string;
}

interface NewSSHRepoCredsParams {
    url: string;
    sshPrivateKey: string;
}

interface NewHTTPSRepoCredsParams {
    url: string;
    username: string;
    password: string;
    tlsClientCertData: string;
    tlsClientCertKey: string;
    proxy: string;
    forceHttpBasicAuth: boolean;
}

interface NewGitHubAppRepoCredsParams {
    url: string;
    githubAppPrivateKey: string;
    githubAppId: bigint;
    githubAppInstallationId: bigint;
    githubAppEnterpriseBaseURL: string;
    tlsClientCertData: string;
    tlsClientCertKey: string;
    proxy: string;
}

interface NewGoogleCloudSourceRepoCredsParams {
    url: string;
    gcpServiceAccountKey: string;
}

export enum ConnectionMethod {
    SSH = 'via SSH',
    HTTPS = 'via HTTPS',
    GITHUBAPP = 'via GitHub App',
    GOOGLECLOUD = 'via Google Cloud'
}

class ReposListComponent extends React.Component<
    RouteComponentProps<any>,
    {
        connecting: boolean;
        method: string;
        currentRepo: models.Repository;
        displayEditPanel: boolean;
    }
> {
    public static contextTypes = {
        router: PropTypes.object,
        apis: PropTypes.object,
        history: PropTypes.object
    };

    private formApi: FormApi;
    private credsTemplate: boolean;
    private repoLoader: DataLoader;
    private credsLoader: DataLoader;

    constructor(props: RouteComponentProps<any>) {
        super(props);
        this.state = {
            connecting: false,
            method: ConnectionMethod.SSH,
            currentRepo: null,
            displayEditPanel: false
        };
    }

    private ConnectRepoFormButton(method: string, onSelection: (method: string) => void) {
        return (
            <div className='white-box'>
                <p>{this.props.t('repos-list.connect.choose-your-connection-method', en['repos-list.connect.choose-your-connection-method'])}</p>
                <DropDownMenu
                    anchor={() => (
                        <p>
                            {method.toUpperCase()} <i className='fa fa-caret-down' />
                        </p>
                    )}
                    items={[ConnectionMethod.SSH, ConnectionMethod.HTTPS, ConnectionMethod.GITHUBAPP, ConnectionMethod.GOOGLECLOUD].map(
                        (connectMethod: ConnectionMethod.SSH | ConnectionMethod.HTTPS | ConnectionMethod.GITHUBAPP | ConnectionMethod.GOOGLECLOUD) => ({
                            title: connectMethod.toUpperCase(),
                            action: () => {
                                onSelection(connectMethod);
                                const formState = this.formApi.getFormState();
                                this.formApi.setFormState({
                                    ...formState,
                                    errors: {}
                                });
                            }
                        })
                    )}
                />
            </div>
        );
    }

    private onChooseDefaultValues = (): FormValues => {
        return {type: 'git', ghType: 'GitHub'};
    };

    private onValidateErrors(params: FormValues): FormErrors {
        switch (this.state.method) {
            case ConnectionMethod.SSH:
                const sshValues = params as NewSSHRepoParams;
                return {
                    url: !sshValues.url && this.props.t('repos-list.connect.ssh.repository-url-is-required', en['repos-list.connect.ssh.repository-url-is-required'])
                };
            case ConnectionMethod.HTTPS:
                const httpsValues = params as NewHTTPSRepoParams;
                return {
                    url:
                        (!httpsValues.url && this.props.t('repos-list.connect.https.repository-url-is-required', en['repos-list.connect.https.repository-url-is-required'])) ||
                        (this.credsTemplate &&
                            !this.isHTTPSUrl(httpsValues.url) &&
                            this.props.t('repos-list.connect.https.not-a-valid-https-url', en['repos-list.connect.https.not-a-valid-https-url'])),
                    name:
                        httpsValues.type === 'helm' &&
                        !httpsValues.name &&
                        this.props.t('repos-list.connect.https.name-is-required', en['repos-list.connect.https.name-is-required']),
                    username:
                        !httpsValues.username &&
                        httpsValues.password &&
                        this.props.t(
                            'repos-list.connect.https.username-is-required-if-password-is-given',
                            en['repos-list.connect.https.username-is-required-if-password-is-given']
                        ),
                    password:
                        !httpsValues.password &&
                        httpsValues.username &&
                        this.props.t(
                            'repos-list.connect.https.username-is-required-if-username-is-given',
                            en['repos-list.connect.https.username-is-required-if-username-is-given']
                        ),
                    tlsClientCertKey:
                        !httpsValues.tlsClientCertKey &&
                        httpsValues.tlsClientCertData &&
                        this.props.t('repos-list.connect.https.tls-client-cert-key-is-required', en['repos-list.connect.https.tls-client-cert-key-is-required'])
                };
            case ConnectionMethod.GITHUBAPP:
                const githubAppValues = params as NewGitHubAppRepoParams;
                return {
                    url:
                        (!githubAppValues.url &&
                            this.props.t('repos-list.connect.github-app.repository-url-is-required', en['repos-list.connect.github-app.repository-url-is-required'])) ||
                        (this.credsTemplate &&
                            !this.isHTTPSUrl(githubAppValues.url) &&
                            this.props.t('repos-list.connect.github-app.not-a-valid-https-url', en['repos-list.connect.github-app.not-a-valid-https-url'])),
                    githubAppId:
                        !githubAppValues.githubAppId &&
                        this.props.t('repos-list.connect.github-app.github-app-id-is-required', en['repos-list.connect.github-app.github-app-id-is-required']),
                    githubAppInstallationId:
                        !githubAppValues.githubAppInstallationId &&
                        this.props.t(
                            'repos-list.connect.github-app.github-app-installation-id-is-required',
                            en['repos-list.connect.github-app.github-app-installation-id-is-required']
                        ),
                    githubAppPrivateKey:
                        !githubAppValues.githubAppPrivateKey &&
                        this.props.t('repos-list.connect.github-app.github-app-private-key-is-required', en['repos-list.connect.github-app.github-app-private-key-is-required'])
                };
            case ConnectionMethod.GOOGLECLOUD:
                const googleCloudValues = params as NewGoogleCloudSourceRepoParams;
                return {
                    url:
                        (!googleCloudValues.url &&
                            this.props.t('repos-list.connect.google-cloud.repo-url-is-required', en['repos-list.connect.google-cloud.repo-url-is-required'])) ||
                        (this.credsTemplate &&
                            !this.isHTTPSUrl(googleCloudValues.url) &&
                            this.props.t('repos-list.connect.google-cloud.not-a-valid-https-url', en['repos-list.connect.google-cloud.not-a-valid-https-url'])),
                    gcpServiceAccountKey:
                        !googleCloudValues.gcpServiceAccountKey &&
                        this.props.t(
                            'repos-list.connect.google-cloud.gcp-service-account-key-is-required',
                            en['repos-list.connect.google-cloud.gcp-service-account-key-is-required']
                        )
                };
        }
    }

    private SlidingPanelHeader() {
        return (
            <>
                {this.showConnectRepo && (
                    <>
                        <button
                            className='argo-button argo-button--base'
                            onClick={() => {
                                this.credsTemplate = false;
                                this.formApi.submitForm(null);
                            }}>
                            <Spinner show={this.state.connecting} style={{marginRight: '5px'}} />
                            {this.props.t('repos-list.sliding-panel.header.connect', en['repos-list.sliding-panel.header.connect'])}
                        </button>{' '}
                        <button
                            className='argo-button argo-button--base'
                            onClick={() => {
                                this.credsTemplate = true;
                                this.formApi.submitForm(null);
                            }}>
                            {this.props.t('repos-list.sliding-panel.header.save-as-credentials-template', en['repos-list.sliding-panel.header.save-as-credentials-template'])}
                        </button>{' '}
                        <button onClick={() => (this.showConnectRepo = false)} className='argo-button argo-button--base-o'>
                            {this.props.t('cancel', en['cancel'])}
                        </button>
                    </>
                )}
                {this.state.displayEditPanel && (
                    <button onClick={() => this.setState({displayEditPanel: false})} className='argo-button argo-button--base-o'>
                        {this.props.t('cancel', en['cancel'])}
                    </button>
                )}
            </>
        );
    }

    private onSubmitForm() {
        switch (this.state.method) {
            case ConnectionMethod.SSH:
                return (params: FormValues) => this.connectSSHRepo(params as NewSSHRepoParams);
            case ConnectionMethod.HTTPS:
                return (params: FormValues) => this.connectHTTPSRepo(params as NewHTTPSRepoParams);
            case ConnectionMethod.GITHUBAPP:
                return (params: FormValues) => this.connectGitHubAppRepo(params as NewGitHubAppRepoParams);
            case ConnectionMethod.GOOGLECLOUD:
                return (params: FormValues) => this.connectGoogleCloudSourceRepo(params as NewGoogleCloudSourceRepoParams);
        }
    }

    public render() {
        return (
            <Page
                title='Repositories'
                toolbar={{
                    breadcrumbs: [
                        {title: this.props.t('repos-list.toolbar.breadcrumbs.0', en['repos-list.toolbar.breadcrumbs.0']), path: '/settings'},
                        {title: this.props.t('repos-list.toolbar.breadcrumbs.1', en['repos-list.toolbar.breadcrumbs.1'])}
                    ],
                    actionMenu: {
                        items: [
                            {
                                iconClassName: 'fa fa-plus',
                                title: this.props.t('repos-list.toolbar.action-menu.connect-repo', en['repos-list.toolbar.action-menu.connect-repo']),
                                action: () => (this.showConnectRepo = true)
                            },
                            {
                                iconClassName: 'fa fa-redo',
                                title: this.props.t('repos-list.toolbar.action-menu.refresh-list', en['repos-list.toolbar.action-menu.refresh-list']),
                                action: () => {
                                    this.refreshRepoList();
                                }
                            }
                        ]
                    }
                }}>
                <div className='repos-list'>
                    <div className='argo-container'>
                        <DataLoader load={() => services.repos.list()} ref={loader => (this.repoLoader = loader)}>
                            {(repos: models.Repository[]) =>
                                (repos.length > 0 && (
                                    <div className='argo-table-list'>
                                        <div className='argo-table-list__head'>
                                            <div className='row'>
                                                <div className='columns small-1' />
                                                <div className='columns small-1'>{this.props.t('repos-list.head.type', en['repos-list.head.type'])}</div>
                                                <div className='columns small-2'>{this.props.t('repos-list.head.name', en['repos-list.head.name'])}</div>
                                                <div className='columns small-5'>{this.props.t('repos-list.head.repository', en['repos-list.head.repository'])}</div>
                                                <div className='columns small-3'>{this.props.t('repos-list.head.connection-state', en['repos-list.head.connection-state'])}</div>
                                            </div>
                                        </div>
                                        {repos.map(repo => (
                                            <div
                                                className={`argo-table-list__row ${this.isRepoUpdatable(repo) ? 'item-clickable' : ''}`}
                                                key={repo.repo}
                                                onClick={() => (this.isRepoUpdatable(repo) ? this.displayEditSliding(repo) : null)}>
                                                <div className='row'>
                                                    <div className='columns small-1'>
                                                        <i className={'icon argo-icon-' + (repo.type || 'git')} />
                                                    </div>
                                                    <div className='columns small-1'>{repo.type || 'git'}</div>
                                                    <div className='columns small-2'>
                                                        <Tooltip content={repo.name}>
                                                            <span>{repo.name}</span>
                                                        </Tooltip>
                                                    </div>
                                                    <div className='columns small-5'>
                                                        <Tooltip content={repo.repo}>
                                                            <span>
                                                                <Repo url={repo.repo} />
                                                            </span>
                                                        </Tooltip>
                                                    </div>
                                                    <div className='columns small-3'>
                                                        <ConnectionStateIcon state={repo.connectionState} /> {repo.connectionState.status}
                                                        <DropDownMenu
                                                            anchor={() => (
                                                                <button className='argo-button argo-button--light argo-button--lg argo-button--short'>
                                                                    <i className='fa fa-ellipsis-v' />
                                                                </button>
                                                            )}
                                                            items={[
                                                                {
                                                                    title: 'Create application',
                                                                    action: () =>
                                                                        this.appContext.apis.navigation.goto('/applications', {
                                                                            new: JSON.stringify({spec: {source: {repoURL: repo.repo}}})
                                                                        })
                                                                },
                                                                {
                                                                    title: 'Disconnect',
                                                                    action: () => this.disconnectRepo(repo.repo)
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
                                        <h4>{this.props.t('repos-list.empty.title', en['repos-list.empty.title'])}</h4>
                                        <h5>{this.props.t('repos-list.empty.description', en['repos-list.empty.description'])}</h5>
                                    </EmptyState>
                                )
                            }
                        </DataLoader>
                    </div>
                    <div className='argo-container'>
                        <DataLoader load={() => services.repocreds.list()} ref={loader => (this.credsLoader = loader)}>
                            {(creds: models.RepoCreds[]) =>
                                creds.length > 0 && (
                                    <div className='argo-table-list'>
                                        <div className='argo-table-list__head'>
                                            <div className='row'>
                                                <div className='columns small-9'>
                                                    {this.props.t('repos-list.credentials-template-url', en['repos-list.credentials-template-url'])}
                                                </div>
                                                <div className='columns small-3'>{this.props.t('repos-list.creds', en['repos-list.creds'])}</div>
                                            </div>
                                        </div>
                                        {creds.map(repo => (
                                            <div className='argo-table-list__row' key={repo.url}>
                                                <div className='row'>
                                                    <div className='columns small-9'>
                                                        <i className='icon argo-icon-git' /> <Repo url={repo.url} />
                                                    </div>
                                                    <div className='columns small-3'>
                                                        -
                                                        <DropDownMenu
                                                            anchor={() => (
                                                                <button className='argo-button argo-button--light argo-button--lg argo-button--short'>
                                                                    <i className='fa fa-ellipsis-v' />
                                                                </button>
                                                            )}
                                                            items={[{title: this.props.t('remove', en['remove']), action: () => this.removeRepoCreds(repo.url)}]}
                                                        />
                                                    </div>
                                                </div>
                                            </div>
                                        ))}
                                    </div>
                                )
                            }
                        </DataLoader>
                    </div>
                </div>
                <SlidingPanel
                    isShown={this.showConnectRepo || this.state.displayEditPanel}
                    onClose={() => {
                        if (!this.state.displayEditPanel && this.showConnectRepo) {
                            this.showConnectRepo = false;
                        }
                        if (this.state.displayEditPanel) {
                            this.setState({displayEditPanel: false});
                        }
                    }}
                    header={this.SlidingPanelHeader()}>
                    {this.showConnectRepo &&
                        this.ConnectRepoFormButton(this.state.method, method => {
                            this.setState({method});
                        })}
                    {this.state.displayEditPanel && <RepoDetails repo={this.state.currentRepo} save={(params: NewHTTPSRepoParams) => this.updateHTTPSRepo(params)} />}
                    {!this.state.displayEditPanel && (
                        <DataLoader load={() => services.projects.list('items.metadata.name').then(projects => projects.map(proj => proj.metadata.name).sort())}>
                            {projects => (
                                <Form
                                    onSubmit={this.onSubmitForm()}
                                    getApi={api => (this.formApi = api)}
                                    defaultValues={this.onChooseDefaultValues()}
                                    validateError={(values: FormValues) => this.onValidateErrors(values)}>
                                    {formApi => (
                                        <form onSubmit={formApi.submitForm} role='form' className='repos-list width-control'>
                                            {this.state.method === ConnectionMethod.SSH && (
                                                <div className='white-box'>
                                                    <p>{this.props.t('repos-list.create.ssh.title', en['repos-list.create.ssh.title'])}</p>
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label={this.props.t(
                                                                'repos-list.create.ssh.name-mandatory-for-helm',
                                                                en['repos-list.create.ssh.name-mandatory-for-helm']
                                                            )}
                                                            field='name'
                                                            component={Text}
                                                        />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label={this.props.t('repos-list.create.fields.project', en['repos-list.create.fields.project'])}
                                                            field='project'
                                                            component={AutocompleteField}
                                                            componentProps={{items: projects}}
                                                        />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label={this.props.t('repos-list.create.fields.repository-url', en['repos-list.create.fields.repository-url'])}
                                                            field='url'
                                                            component={Text}
                                                        />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label={this.props.t('repos-list.create.ssh.ssh-private-key-data', en['repos-list.create.ssh.ssh-private-key-data'])}
                                                            field='sshPrivateKey'
                                                            component={TextArea}
                                                        />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label={this.props.t(
                                                                'repos-list.create.fields.skip-server-verification',
                                                                en['repos-list.create.fields.skip-server-verification']
                                                            )}
                                                            field='insecure'
                                                            component={CheckboxField}
                                                        />
                                                        <HelpIcon
                                                            title={this.props.t(
                                                                'repos-list.create.fields.ignore-create-credential-template',
                                                                en['repos-list.create.fields.ignore-create-credential-template']
                                                            )}
                                                        />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label={this.props.t('repos-list.create.fields.enable-lfs', en['repos-list.create.fields.enable-lfs'])}
                                                            field='enableLfs'
                                                            component={CheckboxField}
                                                        />
                                                        <HelpIcon
                                                            title={this.props.t(
                                                                'repos-list.create.fields.ignore-create-credential-template',
                                                                en['repos-list.create.fields.ignore-create-credential-template']
                                                            )}
                                                        />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label={this.props.t('repos-list.create.fields.proxy', en['repos-list.create.fields.proxy'])}
                                                            field='proxy'
                                                            component={Text}
                                                        />
                                                    </div>
                                                </div>
                                            )}
                                            {this.state.method === ConnectionMethod.HTTPS && (
                                                <div className='white-box'>
                                                    <p>{this.props.t('repos-list.create.https.title', en['repos-list.create.https.title'])}</p>
                                                    <div className='argo-form-row'>
                                                        <FormField formApi={formApi} label='Type' field='type' component={FormSelect} componentProps={{options: ['git', 'helm']}} />
                                                    </div>
                                                    {formApi.getFormState().values.type === 'helm' && (
                                                        <div className='argo-form-row'>
                                                            <FormField
                                                                formApi={formApi}
                                                                label={this.props.t('repos-list.create.https.name', en['repos-list.create.https.name'])}
                                                                field='name'
                                                                component={Text}
                                                            />
                                                        </div>
                                                    )}
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label={this.props.t('repos-list.create.fields.project', en['repos-list.create.fields.project'])}
                                                            field='project'
                                                            component={AutocompleteField}
                                                            componentProps={{items: projects}}
                                                        />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label={this.props.t('repos-list.create.fields.repository-url', en['repos-list.create.fields.repository-url'])}
                                                            field='url'
                                                            component={Text}
                                                        />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label={this.props.t('repos-list.create.https.username', en['repos-list.create.https.username'])}
                                                            field='username'
                                                            component={Text}
                                                        />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label={this.props.t('repos-list.create.https.password', en['repos-list.create.https.password'])}
                                                            field='password'
                                                            component={Text}
                                                            componentProps={{type: 'password'}}
                                                        />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label={this.props.t(
                                                                'repos-list.create.fields.tls-client-certificate',
                                                                en['repos-list.create.fields.tls-client-certificate']
                                                            )}
                                                            field='tlsClientCertData'
                                                            component={TextArea}
                                                        />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label={this.props.t(
                                                                'repos-list.create.fields.tls-client-certificate-key',
                                                                en['repos-list.create.fields.tls-client-certificate-key']
                                                            )}
                                                            field='tlsClientCertKey'
                                                            component={TextArea}
                                                        />
                                                    </div>
                                                    {formApi.getFormState().values.type === 'git' && (
                                                        <React.Fragment>
                                                            <div className='argo-form-row'>
                                                                <FormField
                                                                    formApi={formApi}
                                                                    label={this.props.t(
                                                                        'repos-list.create.fields.skip-server-verification',
                                                                        en['repos-list.create.fields.skip-server-verification']
                                                                    )}
                                                                    field='insecure'
                                                                    component={CheckboxField}
                                                                />
                                                                <HelpIcon
                                                                    title={this.props.t(
                                                                        'repos-list.create.fields.ignore-create-credential-template',
                                                                        en['repos-list.create.fields.ignore-create-credential-template']
                                                                    )}
                                                                />
                                                            </div>
                                                            <div className='argo-form-row'>
                                                                <FormField
                                                                    formApi={formApi}
                                                                    label={this.props.t(
                                                                        'repos-list.create.https.force-http-basic-auth',
                                                                        en['repos-list.create.https.force-http-basic-auth']
                                                                    )}
                                                                    field='forceHttpBasicAuth'
                                                                    component={CheckboxField}
                                                                />
                                                            </div>
                                                            <div className='argo-form-row'>
                                                                <FormField
                                                                    formApi={formApi}
                                                                    label={this.props.t('repos-list.create.fields.enable-lfs', en['repos-list.create.fields.enable-lfs'])}
                                                                    field='enableLfs'
                                                                    component={CheckboxField}
                                                                />
                                                                <HelpIcon
                                                                    title={this.props.t(
                                                                        'repos-list.create.fields.ignore-create-credential-template',
                                                                        en['repos-list.create.fields.ignore-create-credential-template']
                                                                    )}
                                                                />
                                                            </div>
                                                        </React.Fragment>
                                                    )}
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label={this.props.t('repos-list.create.fields.proxy', en['repos-list.create.fields.proxy'])}
                                                            field='proxy'
                                                            component={Text}
                                                        />
                                                    </div>
                                                </div>
                                            )}
                                            {this.state.method === ConnectionMethod.GITHUBAPP && (
                                                <div className='white-box'>
                                                    <p>{this.props.t('repos-list.create.github-app.title', en['repos-list.create.github-app.title'])}</p>
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label={this.props.t('repos-list.create.github-app.type', en['repos-list.create.github-app.type'])}
                                                            field='ghType'
                                                            component={FormSelect}
                                                            componentProps={{options: ['GitHub', 'GitHub Enterprise']}}
                                                        />
                                                    </div>
                                                    {formApi.getFormState().values.ghType === 'GitHub Enterprise' && (
                                                        <React.Fragment>
                                                            <div className='argo-form-row'>
                                                                <FormField
                                                                    formApi={formApi}
                                                                    label={this.props.t(
                                                                        'repos-list.create.github-app.github-enterprise-base-url',
                                                                        en['repos-list.create.github-app.github-enterprise-base-url']
                                                                    )}
                                                                    field='githubAppEnterpriseBaseURL'
                                                                    component={Text}
                                                                />
                                                            </div>
                                                        </React.Fragment>
                                                    )}
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label={this.props.t('repos-list.create.fields.project', en['repos-list.create.fields.project'])}
                                                            field='project'
                                                            component={AutocompleteField}
                                                            componentProps={{items: projects}}
                                                        />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label={this.props.t('repos-list.create.fields.repository-url', en['repos-list.create.fields.repository-url'])}
                                                            field='url'
                                                            component={Text}
                                                        />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label={this.props.t('repos-list.create.github-app.github-app-id', en['repos-list.create.github-app.github-app-id'])}
                                                            field='githubAppId'
                                                            component={NumberField}
                                                        />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label={this.props.t(
                                                                'repos-list.create.github-app.github-app-installation-id',
                                                                en['repos-list.create.github-app.github-app-installation-id']
                                                            )}
                                                            field='githubAppInstallationId'
                                                            component={NumberField}
                                                        />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label={this.props.t(
                                                                'repos-list.create.github-app.github-app-installation-id',
                                                                en['repos-list.create.github-app.github-app-installation-id']
                                                            )}
                                                            field='githubAppPrivateKey'
                                                            component={TextArea}
                                                        />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label={this.props.t(
                                                                'repos-list.create.fields.skip-server-verification',
                                                                en['repos-list.create.fields.skip-server-verification']
                                                            )}
                                                            field='insecure'
                                                            component={CheckboxField}
                                                        />
                                                        <HelpIcon
                                                            title={this.props.t(
                                                                'repos-list.create.fields.ignore-create-credential-template',
                                                                en['repos-list.create.fields.ignore-create-credential-template']
                                                            )}
                                                        />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label={this.props.t('repos-list.create.fields.enable-lfs', en['repos-list.create.fields.enable-lfs'])}
                                                            field='enableLfs'
                                                            component={CheckboxField}
                                                        />
                                                        <HelpIcon
                                                            title={this.props.t(
                                                                'repos-list.create.fields.ignore-create-credential-template',
                                                                en['repos-list.create.fields.ignore-create-credential-template']
                                                            )}
                                                        />
                                                    </div>
                                                    {formApi.getFormState().values.ghType === 'GitHub Enterprise' && (
                                                        <React.Fragment>
                                                            <div className='argo-form-row'>
                                                                <FormField
                                                                    formApi={formApi}
                                                                    label={this.props.t(
                                                                        'repos-list.create.fields.tls-client-certificate',
                                                                        en['repos-list.create.fields.tls-client-certificate']
                                                                    )}
                                                                    field='tlsClientCertData'
                                                                    component={TextArea}
                                                                />
                                                            </div>
                                                            <div className='argo-form-row'>
                                                                <FormField
                                                                    formApi={formApi}
                                                                    label={this.props.t(
                                                                        'repos-list.create.fields.tls-client-certificate-key',
                                                                        en['repos-list.create.fields.tls-client-certificate-key']
                                                                    )}
                                                                    field='tlsClientCertKey'
                                                                    component={TextArea}
                                                                />
                                                            </div>
                                                        </React.Fragment>
                                                    )}
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label={this.props.t('repos-list.create.fields.proxy', en['repos-list.create.fields.proxy'])}
                                                            field='proxy'
                                                            component={Text}
                                                        />
                                                    </div>
                                                </div>
                                            )}
                                            {this.state.method === ConnectionMethod.GOOGLECLOUD && (
                                                <div className='white-box'>
                                                    <p>{this.props.t('repos-list.create.google-cloud.title', en['repos-list.create.google-cloud.title'])}</p>
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label={this.props.t('repos-list.create.fields.project', en['repos-list.create.fields.project'])}
                                                            field='project'
                                                            component={AutocompleteField}
                                                            componentProps={{items: projects}}
                                                        />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label={this.props.t('repos-list.create.fields.repository-url', en['repos-list.create.fields.repository-url'])}
                                                            field='url'
                                                            component={Text}
                                                        />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label={this.props.t(
                                                                'repos-list.create.google-cloud.gcp-service-account-key',
                                                                en['repos-list.create.google-cloud.gcp-service-account-key']
                                                            )}
                                                            field='gcpServiceAccountKey'
                                                            component={TextArea}
                                                        />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label={this.props.t('repos-list.create.fields.proxy', en['repos-list.create.fields.proxy'])}
                                                            field='proxy'
                                                            component={Text}
                                                        />
                                                    </div>
                                                </div>
                                            )}
                                        </form>
                                    )}
                                </Form>
                            )}
                        </DataLoader>
                    )}
                </SlidingPanel>
            </Page>
        );
    }

    private displayEditSliding(repo: models.Repository) {
        this.setState({currentRepo: repo});
        this.setState({displayEditPanel: true});
    }

    // Whether url is a https url (simple version)
    private isHTTPSUrl(url: string) {
        if (url.match(/^https:\/\/.*$/gi)) {
            return true;
        } else {
            return false;
        }
    }

    // only connections of git type which is not via GitHub App are updatable
    private isRepoUpdatable(repo: models.Repository) {
        return this.isHTTPSUrl(repo.repo) && repo.type === 'git' && !repo.githubAppId;
    }

    // Forces a reload of configured repositories, circumventing the cache
    private async refreshRepoList(updatedRepo?: string) {
        try {
            await services.repos.listNoCache();
            await services.repocreds.list();
            this.repoLoader.reload();
            this.appContext.apis.notifications.show({
                content: updatedRepo
                    ? t('repos-list.refresh.updated', en['repos-list.refresh.updated'], {updatedRepo})
                    : t('repos-list.refresh.reloaded', en['repos-list.refresh.reloaded']),
                type: NotificationType.Success
            });
        } catch (e) {
            this.appContext.apis.notifications.show({
                content: <ErrorNotification title={t('repos-list.refresh.failed', en['repos-list.refresh.failed'])} e={e} />,
                type: NotificationType.Error
            });
        }
    }

    // Empty all fields in connect repository form
    private clearConnectRepoForm() {
        this.credsTemplate = false;
        this.formApi.resetAll();
    }

    // Connect a new repository or create a repository credentials for SSH repositories
    private async connectSSHRepo(params: NewSSHRepoParams) {
        if (this.credsTemplate) {
            this.createSSHCreds({url: params.url, sshPrivateKey: params.sshPrivateKey});
        } else {
            this.setState({connecting: true});
            try {
                await services.repos.createSSH(params);
                this.repoLoader.reload();
                this.showConnectRepo = false;
            } catch (e) {
                this.appContext.apis.notifications.show({
                    content: <ErrorNotification title={t('repos-list.connect.ssh.failed', en['repos-list.connect.ssh.failed'])} e={e} />,
                    type: NotificationType.Error
                });
            } finally {
                this.setState({connecting: false});
            }
        }
    }

    // Connect a new repository or create a repository credentials for HTTPS repositories
    private async connectHTTPSRepo(params: NewHTTPSRepoParams) {
        if (this.credsTemplate) {
            this.createHTTPSCreds({
                url: params.url,
                username: params.username,
                password: params.password,
                tlsClientCertData: params.tlsClientCertData,
                tlsClientCertKey: params.tlsClientCertKey,
                proxy: params.proxy,
                forceHttpBasicAuth: params.forceHttpBasicAuth
            });
        } else {
            this.setState({connecting: true});
            try {
                await services.repos.createHTTPS(params);
                this.repoLoader.reload();
                this.showConnectRepo = false;
            } catch (e) {
                this.appContext.apis.notifications.show({
                    content: <ErrorNotification title={t('repos-list.connect.https.failed', en['repos-list.connect.https.failed'])} e={e} />,
                    type: NotificationType.Error
                });
            } finally {
                this.setState({connecting: false});
            }
        }
    }

    // Update an existing repository for HTTPS repositories
    private async updateHTTPSRepo(params: NewHTTPSRepoParams) {
        try {
            await services.repos.updateHTTPS(params);
            this.repoLoader.reload();
            this.setState({displayEditPanel: false});
            this.refreshRepoList(params.url);
        } catch (e) {
            this.appContext.apis.notifications.show({
                content: <ErrorNotification title={t('repos-list.update.https.failed', en['repos-list.update.https.failed'])} e={e} />,
                type: NotificationType.Error
            });
        } finally {
            this.setState({connecting: false});
        }
    }

    // Connect a new repository or create a repository credentials for GitHub App repositories
    private async connectGitHubAppRepo(params: NewGitHubAppRepoParams) {
        if (this.credsTemplate) {
            this.createGitHubAppCreds({
                url: params.url,
                githubAppPrivateKey: params.githubAppPrivateKey,
                githubAppId: params.githubAppId,
                githubAppInstallationId: params.githubAppInstallationId,
                githubAppEnterpriseBaseURL: params.githubAppEnterpriseBaseURL,
                tlsClientCertData: params.tlsClientCertData,
                tlsClientCertKey: params.tlsClientCertKey,
                proxy: params.proxy
            });
        } else {
            this.setState({connecting: true});
            try {
                await services.repos.createGitHubApp(params);
                this.repoLoader.reload();
                this.showConnectRepo = false;
            } catch (e) {
                this.appContext.apis.notifications.show({
                    content: <ErrorNotification title={t('repos-list.connect.github-app.failed', en['repos-list.connect.github-app.failed'])} e={e} />,
                    type: NotificationType.Error
                });
            } finally {
                this.setState({connecting: false});
            }
        }
    }

    // Connect a new repository or create a repository credentials for GitHub App repositories
    private async connectGoogleCloudSourceRepo(params: NewGoogleCloudSourceRepoParams) {
        if (this.credsTemplate) {
            this.createGoogleCloudSourceCreds({
                url: params.url,
                gcpServiceAccountKey: params.gcpServiceAccountKey
            });
        } else {
            this.setState({connecting: true});
            try {
                await services.repos.createGoogleCloudSource(params);
                this.repoLoader.reload();
                this.showConnectRepo = false;
            } catch (e) {
                this.appContext.apis.notifications.show({
                    content: <ErrorNotification title={t('repos-list.connect.google-cloud.failed', en['repos-list.connect.google-cloud.failed'])} e={e} />,
                    type: NotificationType.Error
                });
            } finally {
                this.setState({connecting: false});
            }
        }
    }

    private async createHTTPSCreds(params: NewHTTPSRepoCredsParams) {
        try {
            await services.repocreds.createHTTPS(params);
            this.credsLoader.reload();
            this.showConnectRepo = false;
        } catch (e) {
            this.appContext.apis.notifications.show({
                content: <ErrorNotification title={t('repos-list.create.https.credentials.failed', en['repos-list.create.https.credentials.failed'])} e={e} />,
                type: NotificationType.Error
            });
        }
    }

    private async createSSHCreds(params: NewSSHRepoCredsParams) {
        try {
            await services.repocreds.createSSH(params);
            this.credsLoader.reload();
            this.showConnectRepo = false;
        } catch (e) {
            this.appContext.apis.notifications.show({
                content: <ErrorNotification title={t('repos-list.create.ssh.credentials.failed', en['repos-list.create.ssh.credentials.failed'])} e={e} />,
                type: NotificationType.Error
            });
        }
    }

    private async createGitHubAppCreds(params: NewGitHubAppRepoCredsParams) {
        try {
            await services.repocreds.createGitHubApp(params);
            this.credsLoader.reload();
            this.showConnectRepo = false;
        } catch (e) {
            this.appContext.apis.notifications.show({
                content: <ErrorNotification title={t('repos-list.create.github-app.credentials.failed', en['repos-list.create.github-app.credentials.failed'])} e={e} />,
                type: NotificationType.Error
            });
        }
    }

    private async createGoogleCloudSourceCreds(params: NewGoogleCloudSourceRepoCredsParams) {
        try {
            await services.repocreds.createGoogleCloudSource(params);
            this.credsLoader.reload();
            this.showConnectRepo = false;
        } catch (e) {
            this.appContext.apis.notifications.show({
                content: <ErrorNotification title={t('repos-list.create.google-cloud.credentials.failed', en['repos-list.create.google-cloud.credentials.failed'])} e={e} />,
                type: NotificationType.Error
            });
        }
    }

    // Remove a repository from the configuration
    private async disconnectRepo(repo: string) {
        const confirmed = await this.appContext.apis.popup.confirm(
            t('repos-list.disconnect.repo.title', en['repos-list.disconnect.repo.title']),
            t('repos-list.disconnect.repo.description', en['repos-list.disconnect.repo.description'], {repo})
        );
        if (confirmed) {
            await services.repos.delete(repo);
            this.repoLoader.reload();
        }
    }

    // Remove repository credentials from the configuration
    private async removeRepoCreds(url: string) {
        const confirmed = await this.appContext.apis.popup.confirm(
            t('repos-list.remove-repo-credentials.title', en['repos-list.remove-repo-credentials.title']),
            t('repos-list.remove-repo-credentials.description', en['repos-list.remove-repo-credentials.description'], {url})
        );
        if (confirmed) {
            await services.repocreds.delete(url);
            this.credsLoader.reload();
        }
    }

    // Whether to show the new repository connection dialogue on the page
    private get showConnectRepo() {
        return new URLSearchParams(this.props.location.search).get('addRepo') === 'true';
    }

    private set showConnectRepo(val: boolean) {
        this.clearConnectRepoForm();
        this.appContext.router.history.push(`${this.props.match.url}?addRepo=${val}`);
    }

    private get appContext(): AppContext {
        return this.context as AppContext;
    }
}

export const ReposList = withTranslation()(ReposListComponent);
