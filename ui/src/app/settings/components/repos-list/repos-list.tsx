import {DropDownMenu, FormField, FormSelect, HelpIcon, NotificationType, SlidingPanel} from 'argo-ui';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import {Form, FormApi, Text, TextArea} from 'react-form';
import {RouteComponentProps} from 'react-router';

import {CheckboxField, ConnectionStateIcon, DataLoader, EmptyState, ErrorNotification, NumberField, Page, Repo} from '../../../shared/components';
import {Spinner} from '../../../shared/components';
import {AppContext} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';

require('./repos-list.scss');

interface NewSSHRepoParams {
    type: string;
    name: string;
    url: string;
    sshPrivateKey: string;
    insecure: boolean;
    enableLfs: boolean;
    proxy: string;
}

interface NewHTTPSRepoParams {
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
}

interface NewGitHubAppRepoCredsParams {
    url: string;
    githubAppPrivateKey: string;
    githubAppId: bigint;
    githubAppInstallationId: bigint;
    githubAppEnterpriseBaseURL: string;
    tlsClientCertData: string;
    tlsClientCertKey: string;
}

export class ReposList extends React.Component<RouteComponentProps<any>, {connecting: boolean}> {
    public static contextTypes = {
        router: PropTypes.object,
        apis: PropTypes.object,
        history: PropTypes.object
    };

    private formApiSSH: FormApi;
    private formApiHTTPS: FormApi;
    private formApiGitHubApp: FormApi;
    private credsTemplate: boolean;
    private repoLoader: DataLoader;
    private credsLoader: DataLoader;

    constructor(props: RouteComponentProps<any>) {
        super(props);
        this.state = {connecting: false};
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
                                title: 'Connect Repo using SSH',
                                action: () => (this.showConnectSSHRepo = true)
                            },
                            {
                                iconClassName: 'fa fa-plus',
                                title: 'Connect Repo using HTTPS',
                                action: () => (this.showConnectHTTPSRepo = true)
                            },
                            {
                                iconClassName: 'fa fa-plus',
                                title: 'Connect Repo using GitHub App',
                                action: () => (this.showConnectGitHubAppRepo = true)
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
                                                <div className='columns small-5'>REPOSITORY</div>
                                                <div className='columns small-3'>CONNECTION STATUS</div>
                                            </div>
                                        </div>
                                        {repos.map(repo => (
                                            <div className='argo-table-list__row' key={repo.repo}>
                                                <div className='row'>
                                                    <div className='columns small-1'>
                                                        <i className={'icon argo-icon-' + (repo.type || 'git')} />
                                                    </div>
                                                    <div className='columns small-1'>{repo.type || 'git'}</div>
                                                    <div className='columns small-2'>{repo.name}</div>
                                                    <div className='columns small-5'>
                                                        <Repo url={repo.repo} />
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
                                        <h4>No repositories connected</h4>
                                        <h5>Connect your repo to deploy apps.</h5>
                                        <button className='argo-button argo-button--base' onClick={() => (this.showConnectSSHRepo = true)}>
                                            Connect Repo using SSH
                                        </button>{' '}
                                        <button className='argo-button argo-button--base' onClick={() => (this.showConnectHTTPSRepo = true)}>
                                            Connect Repo using HTTPS
                                        </button>
                                        <button className='argo-button argo-button--base' onClick={() => (this.showConnectGitHubAppRepo = true)}>
                                            Connect Repo using GitHub App
                                        </button>{' '}
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
                    isShown={this.showConnectHTTPSRepo}
                    onClose={() => (this.showConnectHTTPSRepo = false)}
                    header={
                        <div>
                            <button
                                className='argo-button argo-button--base'
                                onClick={() => {
                                    this.credsTemplate = false;
                                    this.formApiHTTPS.submitForm(null);
                                }}>
                                <Spinner show={this.state.connecting} style={{marginRight: '5px'}} />
                                Connect
                            </button>{' '}
                            <button
                                className='argo-button argo-button--base'
                                onClick={() => {
                                    this.credsTemplate = true;
                                    this.formApiHTTPS.submitForm(null);
                                }}>
                                Save as credentials template
                            </button>{' '}
                            <button onClick={() => (this.showConnectHTTPSRepo = false)} className='argo-button argo-button--base-o'>
                                Cancel
                            </button>
                        </div>
                    }>
                    <h4>Connect repo using HTTPS</h4>
                    <Form
                        onSubmit={params => this.connectHTTPSRepo(params as NewHTTPSRepoParams)}
                        getApi={api => (this.formApiHTTPS = api)}
                        defaultValues={{type: 'git'}}
                        validateError={(params: NewHTTPSRepoParams) => ({
                            url: (!params.url && 'Repo URL is required') || (this.credsTemplate && !this.isHTTPSUrl(params.url) && 'Not a valid HTTPS URL'),
                            name: params.type === 'helm' && !params.name && 'Name is required',
                            password: !params.password && params.username && 'Password is required if username is given.',
                            tlsClientCertKey: !params.tlsClientCertKey && params.tlsClientCertData && 'TLS client cert key is required if TLS client cert is given.'
                        })}>
                        {formApi => (
                            <form onSubmit={formApi.submitForm} role='form' className='repos-list width-control'>
                                <div className='argo-form-row'>
                                    <FormField formApi={formApi} label='Type' field='type' component={FormSelect} componentProps={{options: ['git', 'helm']}} />
                                </div>
                                {formApi.getFormState().values.type === 'helm' && (
                                    <div className='argo-form-row'>
                                        <FormField formApi={formApi} label='Name' field='name' component={Text} />
                                    </div>
                                )}
                                <div className='argo-form-row'>
                                    <FormField formApi={formApi} label='Repository URL' field='url' component={Text} />
                                </div>
                                <div className='argo-form-row'>
                                    <FormField formApi={formApi} label='Username (optional)' field='username' component={Text} />
                                </div>
                                <div className='argo-form-row'>
                                    <FormField formApi={formApi} label='Password (optional)' field='password' component={Text} componentProps={{type: 'password'}} />
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
                                            <FormField formApi={formApi} label='Enable LFS support (Git only)' field='enableLfs' component={CheckboxField} />
                                            <HelpIcon title='This setting is ignored when creating as credential template.' />
                                        </div>
                                    </React.Fragment>
                                )}
                                <div className='argo-form-row'>
                                    <FormField formApi={formApi} label='Proxy (optional)' field='proxy' component={Text} />
                                </div>
                            </form>
                        )}
                    </Form>
                </SlidingPanel>
                <SlidingPanel
                    isShown={this.showConnectSSHRepo}
                    onClose={() => (this.showConnectSSHRepo = false)}
                    header={
                        <div>
                            <button
                                className='argo-button argo-button--base'
                                onClick={() => {
                                    this.credsTemplate = false;
                                    this.formApiSSH.submitForm(null);
                                }}>
                                <Spinner show={this.state.connecting} style={{marginRight: '5px'}} />
                                Connect
                            </button>{' '}
                            <button
                                className='argo-button argo-button--base'
                                onClick={() => {
                                    this.credsTemplate = true;
                                    this.formApiSSH.submitForm(null);
                                }}>
                                Save as credentials template
                            </button>{' '}
                            <button onClick={() => (this.showConnectSSHRepo = false)} className='argo-button argo-button--base-o'>
                                Cancel
                            </button>
                        </div>
                    }>
                    <h4>Connect repo using SSH</h4>
                    <Form
                        onSubmit={params => this.connectSSHRepo(params as NewSSHRepoParams)}
                        getApi={api => (this.formApiSSH = api)}
                        defaultValues={{type: 'git'}}
                        validateError={(params: NewSSHRepoParams) => ({
                            url: !params.url && 'Repo URL is required'
                        })}>
                        {formApi => (
                            <form onSubmit={formApi.submitForm} role='form' className='repos-list width-control'>
                                <div className='argo-form-row'>
                                    <FormField formApi={formApi} label='Name (mandatory for Helm)' field='name' component={Text} />
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
                            </form>
                        )}
                    </Form>
                </SlidingPanel>
                <SlidingPanel
                    isShown={this.showConnectGitHubAppRepo}
                    onClose={() => (this.showConnectGitHubAppRepo = false)}
                    header={
                        <div>
                            <button
                                className='argo-button argo-button--base'
                                onClick={() => {
                                    this.credsTemplate = false;
                                    this.formApiGitHubApp.submitForm(null);
                                }}>
                                <Spinner show={this.state.connecting} style={{marginRight: '5px'}} />
                                Connect
                            </button>{' '}
                            <button
                                className='argo-button argo-button--base'
                                onClick={() => {
                                    this.credsTemplate = true;
                                    this.formApiGitHubApp.submitForm(null);
                                }}>
                                Save as credentials template
                            </button>{' '}
                            <button onClick={() => (this.showConnectGitHubAppRepo = false)} className='argo-button argo-button--base-o'>
                                Cancel
                            </button>
                        </div>
                    }>
                    <h4>Connect repo using GitHub App</h4>
                    <Form
                        onSubmit={params => this.connectGitHubAppRepo(params as NewGitHubAppRepoParams)}
                        getApi={api => (this.formApiGitHubApp = api)}
                        defaultValues={{type: 'git', ghType: 'GitHub'}}
                        validateError={(params: NewGitHubAppRepoParams) => ({
                            url: (!params.url && 'Repo URL is required') || (this.credsTemplate && !this.isHTTPSUrl(params.url) && 'Not a valid HTTPS URL'),
                            githubAppId: !params.githubAppId && 'GitHub App ID is required',
                            githubAppInstallationId: !params.githubAppInstallationId && 'GitHub App installation ID is required',
                            githubAppPrivateKey: !params.githubAppPrivateKey && 'GitHub App private Key is required'
                        })}>
                        {formApi => (
                            <form onSubmit={formApi.submitForm} role='form' className='repos-list width-control'>
                                <div className='argo-form-row'>
                                    <FormField formApi={formApi} label='Type' field='ghType' component={FormSelect} componentProps={{options: ['GitHub', 'GitHub Enterprise']}} />
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
                                            <FormField formApi={formApi} label='TLS client certificate (optional)' field='tlsClientCertData' component={TextArea} />
                                        </div>
                                        <div className='argo-form-row'>
                                            <FormField formApi={formApi} label='TLS client certificate key (optional)' field='tlsClientCertKey' component={TextArea} />
                                        </div>
                                    </React.Fragment>
                                )}
                                <div className='argo-form-row'>
                                    <FormField formApi={formApi} label='Proxy (optional)' field='proxy' component={Text} />
                                </div>
                            </form>
                        )}
                    </Form>
                </SlidingPanel>
            </Page>
        );
    }

    // Whether url is a https url (simple version)
    private isHTTPSUrl(url: string) {
        if (url.match(/^https:\/\/.*$/gi)) {
            return true;
        } else {
            return false;
        }
    }

    // Forces a reload of configured repositories, circumventing the cache
    private async refreshRepoList() {
        try {
            await services.repos.listNoCache();
            await services.repocreds.list();
            this.repoLoader.reload();
            this.appContext.apis.notifications.show({
                content: 'Successfully reloaded list of repositories',
                type: NotificationType.Success
            });
        } catch (e) {
            this.appContext.apis.notifications.show({
                content: <ErrorNotification title='Could not refresh list of repositories' e={e} />,
                type: NotificationType.Error
            });
        }
    }

    // Empty all fields in SSH repository form
    private clearConnectSSHForm() {
        this.credsTemplate = false;
        this.formApiSSH.resetAll();
    }

    // Empty all fields in HTTPS repository form
    private clearConnectHTTPSForm() {
        this.credsTemplate = false;
        this.formApiHTTPS.resetAll();
    }

    // Empty all fields in SSH repository form
    private clearConnectGitHubAppForm() {
        this.credsTemplate = false;
        this.formApiGitHubApp.resetAll();
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
                this.showConnectSSHRepo = false;
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
                tlsClientCertKey: params.tlsClientCertKey
            });
        } else {
            this.setState({connecting: true});
            try {
                await services.repos.createHTTPS(params);
                this.repoLoader.reload();
                this.showConnectHTTPSRepo = false;
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
                tlsClientCertKey: params.tlsClientCertKey
            });
        } else {
            this.setState({connecting: true});
            try {
                await services.repos.createGitHubApp(params);
                this.repoLoader.reload();
                this.showConnectGitHubAppRepo = false;
            } catch (e) {
                this.appContext.apis.notifications.show({
                    content: <ErrorNotification title='Unable to connect GitHub app repository' e={e} />,
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
            this.showConnectHTTPSRepo = false;
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
            this.showConnectSSHRepo = false;
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
            this.showConnectGitHubAppRepo = false;
        } catch (e) {
            this.appContext.apis.notifications.show({
                content: <ErrorNotification title='Unable to create GitHub App credentials' e={e} />,
                type: NotificationType.Error
            });
        }
    }

    // Remove a repository from the configuration
    private async disconnectRepo(repo: string) {
        const confirmed = await this.appContext.apis.popup.confirm('Disconnect repository', `Are you sure you want to disconnect '${repo}'?`);
        if (confirmed) {
            await services.repos.delete(repo);
            this.repoLoader.reload();
        }
    }

    // Remove repository credentials from the configuration
    private async removeRepoCreds(url: string) {
        const confirmed = await this.appContext.apis.popup.confirm('Remove repository credentials', `Are you sure you want to remove credentials for URL prefix '${url}'?`);
        if (confirmed) {
            await services.repocreds.delete(url);
            this.credsLoader.reload();
        }
    }

    // Whether to show the HTTPS repository connection dialogue on the page
    private get showConnectHTTPSRepo() {
        return new URLSearchParams(this.props.location.search).get('addHTTPSRepo') === 'true';
    }

    private set showConnectHTTPSRepo(val: boolean) {
        this.clearConnectHTTPSForm();
        this.appContext.router.history.push(`${this.props.match.url}?addHTTPSRepo=${val}`);
    }

    // Whether to show the SSH repository connection dialogue on the page
    private get showConnectSSHRepo() {
        return new URLSearchParams(this.props.location.search).get('addSSHRepo') === 'true';
    }

    private set showConnectSSHRepo(val: boolean) {
        this.clearConnectSSHForm();
        this.appContext.router.history.push(`${this.props.match.url}?addSSHRepo=${val}`);
    }

    // Whether to show the GitHub App repository connection dialogue on the page
    private get showConnectGitHubAppRepo() {
        return new URLSearchParams(this.props.location.search).get('addGitHubAppRepo') === 'true';
    }

    private set showConnectGitHubAppRepo(val: boolean) {
        this.clearConnectGitHubAppForm();
        this.appContext.router.history.push(`${this.props.match.url}?addGitHubAppRepo=${val}`);
    }

    private get appContext(): AppContext {
        return this.context as AppContext;
    }
}
