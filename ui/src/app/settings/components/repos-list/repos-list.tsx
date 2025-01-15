/* eslint-disable no-case-declarations */
import {AutocompleteField, DropDownMenu, FormField, FormSelect, HelpIcon, NotificationType, SlidingPanel, Tooltip} from 'argo-ui';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import {Form, FormValues, FormApi, Text, TextArea, FormErrors} from 'react-form';
import {RouteComponentProps} from 'react-router';

import {CheckboxField, ConnectionStateIcon, DataLoader, EmptyState, ErrorNotification, NumberField, Page, Repo, Spinner} from '../../../shared/components';
import {AppContext} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {RepoDetails} from '../repo-details/repo-details';

require('./repos-list.scss');

interface NewSSHRepoParams {
    type: string;
    name: string;
    url: string;
    sshPrivateKey: string;
    insecure: boolean;
    enableLfs: boolean;
    proxy: string;
    noProxy: string;
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
    noProxy: string;
    project?: string;
    forceHttpBasicAuth?: boolean;
    enableOCI: boolean;
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
    noProxy: string;
    project?: string;
}

interface NewGoogleCloudSourceRepoParams {
    type: string;
    name: string;
    url: string;
    gcpServiceAccountKey: string;
    proxy: string;
    noProxy: string;
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
    noProxy: string;
    forceHttpBasicAuth: boolean;
    enableOCI: boolean;
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
    noProxy: string;
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

export class ReposList extends React.Component<
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
                <p>Choose your connection method:</p>
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
                    url: !sshValues.url && 'Repository URL is required'
                };
            case ConnectionMethod.HTTPS:
                const httpsValues = params as NewHTTPSRepoParams;
                return {
                    url:
                        (!httpsValues.url && 'Repository URL is required') ||
                        (this.credsTemplate && !this.isHTTPSUrl(httpsValues.url) && !httpsValues.enableOCI && 'Not a valid HTTPS URL'),
                    name: httpsValues.type === 'helm' && !httpsValues.name && 'Name is required',
                    username: !httpsValues.username && httpsValues.password && 'Username is required if password is given.',
                    password: !httpsValues.password && httpsValues.username && 'Password is required if username is given.',
                    tlsClientCertKey: !httpsValues.tlsClientCertKey && httpsValues.tlsClientCertData && 'TLS client cert key is required if TLS client cert is given.'
                };
            case ConnectionMethod.GITHUBAPP:
                const githubAppValues = params as NewGitHubAppRepoParams;
                return {
                    url: (!githubAppValues.url && 'Repository URL is required') || (this.credsTemplate && !this.isHTTPSUrl(githubAppValues.url) && 'Not a valid HTTPS URL'),
                    githubAppId: !githubAppValues.githubAppId && 'GitHub App ID is required',
                    githubAppInstallationId: !githubAppValues.githubAppInstallationId && 'GitHub App installation ID is required',
                    githubAppPrivateKey: !githubAppValues.githubAppPrivateKey && 'GitHub App private Key is required'
                };
            case ConnectionMethod.GOOGLECLOUD:
                const googleCloudValues = params as NewGoogleCloudSourceRepoParams;
                return {
                    url: (!googleCloudValues.url && 'Repo URL is required') || (this.credsTemplate && !this.isHTTPSUrl(googleCloudValues.url) && 'Not a valid HTTPS URL'),
                    gcpServiceAccountKey: !googleCloudValues.gcpServiceAccountKey && 'GCP service account key is required'
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
                            Connect
                        </button>{' '}
                        <button
                            className='argo-button argo-button--base'
                            onClick={() => {
                                this.credsTemplate = true;
                                this.formApi.submitForm(null);
                            }}>
                            Save as credentials template
                        </button>{' '}
                        <button onClick={() => (this.showConnectRepo = false)} className='argo-button argo-button--base-o'>
                            Cancel
                        </button>
                    </>
                )}
                {this.state.displayEditPanel && (
                    <button onClick={() => this.setState({displayEditPanel: false})} className='argo-button argo-button--base-o'>
                        Cancel
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
                return (params: FormValues) => {
                    params.url = params.enableOCI ? this.stripProtocol(params.url) : params.url;
                    return this.connectHTTPSRepo(params as NewHTTPSRepoParams);
                };
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
                    breadcrumbs: [{title: 'Settings', path: '/settings'}, {title: 'Repositories'}],
                    actionMenu: {
                        items: [
                            {
                                iconClassName: 'fa fa-plus',
                                title: 'Connect Repo',
                                action: () => (this.showConnectRepo = true)
                            },
                            {
                                iconClassName: 'fa fa-redo',
                                title: 'Refresh list',
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
                                                <div className='columns small-1'>TYPE</div>
                                                <div className='columns small-2'>NAME</div>
                                                <div className='columns small-2'>PROJECT</div>
                                                <div className='columns small-4'>REPOSITORY</div>
                                                <div className='columns small-2'>CONNECTION STATUS</div>
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
                                                    <div className='columns small-1'>
                                                        <span>{repo.type || 'git'}</span>
                                                        {repo.enableOCI && <span> OCI</span>}
                                                    </div>
                                                    <div className='columns small-2'>
                                                        <Tooltip content={repo.name}>
                                                            <span>{repo.name}</span>
                                                        </Tooltip>
                                                    </div>
                                                    <div className='columns small-2'>
                                                        <Tooltip content={repo.project}>
                                                            <span>{repo.project}</span>
                                                        </Tooltip>
                                                    </div>
                                                    <div className='columns small-4'>
                                                        <Tooltip content={repo.repo}>
                                                            <span>
                                                                <Repo url={repo.repo} />
                                                            </span>
                                                        </Tooltip>
                                                    </div>
                                                    <div className='columns small-2'>
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
                                                                    action: () => this.disconnectRepo(repo.repo, repo.project)
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
                                        <h4>No repositories connected</h4>
                                        <h5>Connect your repo to deploy apps.</h5>
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
                                                <div className='columns small-9'>CREDENTIALS TEMPLATE URL</div>
                                                <div className='columns small-3'>CREDS</div>
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
                                                            items={[{title: 'Remove', action: () => this.removeRepoCreds(repo.url)}]}
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
                                                    <p>CONNECT REPO USING SSH</p>
                                                    <div className='argo-form-row'>
                                                        <FormField formApi={formApi} label='Name (mandatory for Helm)' field='name' component={Text} />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label='Project'
                                                            field='project'
                                                            component={AutocompleteField}
                                                            componentProps={{items: projects}}
                                                        />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField formApi={formApi} label='Repository URL' field='url' component={Text} />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField formApi={formApi} label='SSH private key data' field='sshPrivateKey' component={TextArea} />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField formApi={formApi} label='Skip server verification' field='insecure' component={CheckboxField} />
                                                        <HelpIcon title='This setting is ignored when creating as credential template.' />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField formApi={formApi} label='Enable LFS support (Git only)' field='enableLfs' component={CheckboxField} />
                                                        <HelpIcon title='This setting is ignored when creating as credential template.' />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField formApi={formApi} label='Proxy (optional)' field='proxy' component={Text} />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField formApi={formApi} label='NoProxy (optional)' field='noProxy' component={Text} />
                                                    </div>
                                                </div>
                                            )}
                                            {this.state.method === ConnectionMethod.HTTPS && (
                                                <div className='white-box'>
                                                    <p>CONNECT REPO USING HTTPS</p>
                                                    <div className='argo-form-row'>
                                                        <FormField formApi={formApi} label='Type' field='type' component={FormSelect} componentProps={{options: ['git', 'helm']}} />
                                                    </div>
                                                    {formApi.getFormState().values.type === 'helm' && (
                                                        <div className='argo-form-row'>
                                                            <FormField formApi={formApi} label='Name' field='name' component={Text} />
                                                        </div>
                                                    )}
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label='Project'
                                                            field='project'
                                                            component={AutocompleteField}
                                                            componentProps={{items: projects}}
                                                        />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField formApi={formApi} label='Repository URL' field='url' component={Text} />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField formApi={formApi} label='Username (optional)' field='username' component={Text} />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label='Password (optional)'
                                                            field='password'
                                                            component={Text}
                                                            componentProps={{type: 'password'}}
                                                        />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField formApi={formApi} label='TLS client certificate (optional)' field='tlsClientCertData' component={TextArea} />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField formApi={formApi} label='TLS client certificate key (optional)' field='tlsClientCertKey' component={TextArea} />
                                                    </div>
                                                    {formApi.getFormState().values.type === 'git' && (
                                                        <React.Fragment>
                                                            <div className='argo-form-row'>
                                                                <FormField formApi={formApi} label='Skip server verification' field='insecure' component={CheckboxField} />
                                                                <HelpIcon title='This setting is ignored when creating as credential template.' />
                                                            </div>
                                                            <div className='argo-form-row'>
                                                                <FormField formApi={formApi} label='Force HTTP basic auth' field='forceHttpBasicAuth' component={CheckboxField} />
                                                            </div>
                                                            <div className='argo-form-row'>
                                                                <FormField formApi={formApi} label='Enable LFS support (Git only)' field='enableLfs' component={CheckboxField} />
                                                                <HelpIcon title='This setting is ignored when creating as credential template.' />
                                                            </div>
                                                        </React.Fragment>
                                                    )}
                                                    <div className='argo-form-row'>
                                                        <FormField formApi={formApi} label='Proxy (optional)' field='proxy' component={Text} />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField formApi={formApi} label='NoProxy (optional)' field='noProxy' component={Text} />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField formApi={formApi} label='Enable OCI' field='enableOCI' component={CheckboxField} />
                                                    </div>
                                                </div>
                                            )}
                                            {this.state.method === ConnectionMethod.GITHUBAPP && (
                                                <div className='white-box'>
                                                    <p>CONNECT REPO USING GITHUB APP</p>
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label='Type'
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
                                                                    label='GitHub Enterprise Base URL (e.g. https://ghe.example.com/api/v3)'
                                                                    field='githubAppEnterpriseBaseURL'
                                                                    component={Text}
                                                                />
                                                            </div>
                                                        </React.Fragment>
                                                    )}
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label='Project'
                                                            field='project'
                                                            component={AutocompleteField}
                                                            componentProps={{items: projects}}
                                                        />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField formApi={formApi} label='Repository URL' field='url' component={Text} />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField formApi={formApi} label='GitHub App ID' field='githubAppId' component={NumberField} />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField formApi={formApi} label='GitHub App Installation ID' field='githubAppInstallationId' component={NumberField} />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField formApi={formApi} label='GitHub App private key' field='githubAppPrivateKey' component={TextArea} />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField formApi={formApi} label='Skip server verification' field='insecure' component={CheckboxField} />
                                                        <HelpIcon title='This setting is ignored when creating as credential template.' />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField formApi={formApi} label='Enable LFS support (Git only)' field='enableLfs' component={CheckboxField} />
                                                        <HelpIcon title='This setting is ignored when creating as credential template.' />
                                                    </div>
                                                    {formApi.getFormState().values.ghType === 'GitHub Enterprise' && (
                                                        <React.Fragment>
                                                            <div className='argo-form-row'>
                                                                <FormField
                                                                    formApi={formApi}
                                                                    label='TLS client certificate (optional)'
                                                                    field='tlsClientCertData'
                                                                    component={TextArea}
                                                                />
                                                            </div>
                                                            <div className='argo-form-row'>
                                                                <FormField
                                                                    formApi={formApi}
                                                                    label='TLS client certificate key (optional)'
                                                                    field='tlsClientCertKey'
                                                                    component={TextArea}
                                                                />
                                                            </div>
                                                        </React.Fragment>
                                                    )}
                                                    <div className='argo-form-row'>
                                                        <FormField formApi={formApi} label='Proxy (optional)' field='proxy' component={Text} />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField formApi={formApi} label='NoProxy (optional)' field='noProxy' component={Text} />
                                                    </div>
                                                </div>
                                            )}
                                            {this.state.method === ConnectionMethod.GOOGLECLOUD && (
                                                <div className='white-box'>
                                                    <p>CONNECT REPO USING GOOGLE CLOUD</p>
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label='Project'
                                                            field='project'
                                                            component={AutocompleteField}
                                                            componentProps={{items: projects}}
                                                        />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField formApi={formApi} label='Repository URL' field='url' component={Text} />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField formApi={formApi} label='GCP service account key' field='gcpServiceAccountKey' component={TextArea} />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField formApi={formApi} label='Proxy (optional)' field='proxy' component={Text} />
                                                    </div>
                                                    <div className='argo-form-row'>
                                                        <FormField formApi={formApi} label='NoProxy (optional)' field='noProxy' component={Text} />
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

    private stripProtocol(url: string) {
        return url.replace('https://', '').replace('oci://', '');
    }

    // only connections of git type which is not via GitHub App are updatable
    private isRepoUpdatable(repo: models.Repository) {
        return this.isHTTPSUrl(repo.repo) && repo.type === 'git' && !repo.githubAppId;
    }

    // Forces a reload of configured repositories, circumventing the cache
    private async refreshRepoList(updatedRepo?: string) {
        // Refresh the credentials template list
        this.credsLoader.reload();

        try {
            await services.repos.listNoCache();
            this.repoLoader.reload();
            this.appContext.apis.notifications.show({
                content: updatedRepo ? `Successfully updated ${updatedRepo} repository` : 'Successfully reloaded list of repositories',
                type: NotificationType.Success
            });
        } catch (e) {
            this.appContext.apis.notifications.show({
                content: <ErrorNotification title='Could not refresh list of repositories' e={e} />,
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
                    content: <ErrorNotification title='Unable to connect SSH repository' e={e} />,
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
                noProxy: params.noProxy,
                forceHttpBasicAuth: params.forceHttpBasicAuth,
                enableOCI: params.enableOCI
            });
        } else {
            this.setState({connecting: true});
            try {
                await services.repos.createHTTPS(params);
                this.repoLoader.reload();
                this.showConnectRepo = false;
            } catch (e) {
                this.appContext.apis.notifications.show({
                    content: <ErrorNotification title='Unable to connect HTTPS repository' e={e} />,
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
                content: <ErrorNotification title='Unable to update HTTPS repository' e={e} />,
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
                proxy: params.proxy,
                noProxy: params.noProxy
            });
        } else {
            this.setState({connecting: true});
            try {
                await services.repos.createGitHubApp(params);
                this.repoLoader.reload();
                this.showConnectRepo = false;
            } catch (e) {
                this.appContext.apis.notifications.show({
                    content: <ErrorNotification title='Unable to connect GitHub App repository' e={e} />,
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
                    content: <ErrorNotification title='Unable to connect Google Cloud Source repository' e={e} />,
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
                content: <ErrorNotification title='Unable to create HTTPS credentials' e={e} />,
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
                content: <ErrorNotification title='Unable to create SSH credentials' e={e} />,
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
                content: <ErrorNotification title='Unable to create GitHub App credentials' e={e} />,
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
                content: <ErrorNotification title='Unable to create Google Cloud Source credentials' e={e} />,
                type: NotificationType.Error
            });
        }
    }

    // Remove a repository from the configuration
    private async disconnectRepo(repo: string, project: string) {
        const confirmed = await this.appContext.apis.popup.confirm('Disconnect repository', `Are you sure you want to disconnect '${repo}'?`);
        if (confirmed) {
            try {
                await services.repos.delete(repo, project || '');
                this.repoLoader.reload();
            } catch (e) {
                this.appContext.apis.notifications.show({
                    content: <ErrorNotification title='Unable to disconnect repository' e={e} />,
                    type: NotificationType.Error
                });
            }
        }
    }

    // Remove repository credentials from the configuration
    private async removeRepoCreds(url: string) {
        const confirmed = await this.appContext.apis.popup.confirm('Remove repository credentials', `Are you sure you want to remove credentials for URL prefix '${url}'?`);
        if (confirmed) {
            try {
                await services.repocreds.delete(url);
                this.credsLoader.reload();
            } catch (e) {
                this.appContext.apis.notifications.show({
                    content: <ErrorNotification title='Unable to remove repository credentials' e={e} />,
                    type: NotificationType.Error
                });
            }
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
