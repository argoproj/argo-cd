/* eslint-disable no-case-declarations */
import {AutocompleteField, DropDownMenu, FormField, FormSelect, HelpIcon, NotificationType, SlidingPanel, Tooltip} from 'argo-ui';
import React, {useEffect, useState, useContext, useRef} from 'react';
import {withRouter, RouteComponentProps} from 'react-router-dom';
import {Form, FormValues, FormApi, Text, TextArea, FormErrors} from 'react-form';
import {Context} from '../../../shared/context';
import {CheckboxField, ConnectionStateIcon, DataLoader, EmptyState, ErrorNotification, NumberField, Page, Repo, Spinner} from '../../../shared/components';
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
    // write should be true if saving as a write credential.
    write: boolean;
    sparsePaths: string;
    enablePartialClone: boolean;
}

export interface NewHTTPSRepoParams {
    type: string;
    name: string;
    url: string;
    username: string;
    password: string;
    bearerToken: string;
    tlsClientCertData: string;
    tlsClientCertKey: string;
    insecure: boolean;
    enableLfs: boolean;
    proxy: string;
    noProxy: string;
    project?: string;
    forceHttpBasicAuth?: boolean;
    enableOCI: boolean;
    insecureOCIForceHttp: boolean;
    // write should be true if saving as a write credential.
    write: boolean;
    useAzureWorkloadIdentity: boolean;
    sparsePaths: string;
    enablePartialClone: boolean;
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
    // write should be true if saving as a write credential.
    write: boolean;
    sparsePaths: string;
    enablePartialClone: boolean;
}

interface NewGoogleCloudSourceRepoParams {
    type: string;
    name: string;
    url: string;
    gcpServiceAccountKey: string;
    proxy: string;
    noProxy: string;
    project?: string;
    // write should be true if saving as a write credential.
    write: boolean;
}

interface NewSSHRepoCredsParams {
    url: string;
    sshPrivateKey: string;
    // write should be true if saving as a write credential.
    write: boolean;
    sparsePaths?: string;
    enablePartialClone?: boolean;
}

interface NewHTTPSRepoCredsParams {
    url: string;
    type: string;
    username: string;
    password: string;
    bearerToken: string;
    tlsClientCertData: string;
    tlsClientCertKey: string;
    proxy: string;
    noProxy: string;
    forceHttpBasicAuth: boolean;
    enableOCI: boolean;
    insecureOCIForceHttp: boolean;
    // write should be true if saving as a write credential.
    write: boolean;
    useAzureWorkloadIdentity: boolean;
    enablePartialClone?: boolean;
    sparsePaths?: string;
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
    // write should be true if saving as a write credential.
    write: boolean;
}

interface NewGoogleCloudSourceRepoCredsParams {
    url: string;
    gcpServiceAccountKey: string;
    // write should be true if saving as a write credential.
    write: boolean;
}

export enum ConnectionMethod {
    SSH = 'via SSH',
    HTTPS = 'via HTTP/HTTPS',
    GITHUBAPP = 'via GitHub App',
    GOOGLECLOUD = 'via Google Cloud'
}

export const ReposList = ({match, location}: RouteComponentProps) => {
    const [connecting, setConnecting] = useState(false);
    const [method, setMethod] = useState<string>(ConnectionMethod.SSH);
    const [currentRepo, setCurrentRepo] = useState<models.Repository | null>(null);
    const [displayEditPanel, setDisplayEditPanel] = useState(false);
    const [authSettings, setAuthSettings] = useState<models.AuthSettings | null>(null);
    // States related to repository sorting
    const [statusProperty, setStatusProperty] = useState<'all' | 'Successful' | 'Failed' | 'Unknown'>('all');
    const [projectProperty, setProjectProperty] = useState<string>('all');
    const [typeProperty, setTypeProperty] = useState<'all' | 'git' | 'helm'>('all');
    const [name, setName] = useState<string>('');

    const ctx = useContext(Context);

    const formApi = useRef<FormApi | null>(null);
    const credsTemplate = useRef<boolean>(false);
    const repoLoader = useRef<DataLoader | null>(null);
    const credsLoader = useRef<DataLoader | null>(null);

    useEffect(() => {
        const fetchAuthSettings = async () => {
            const settings = await services.authService.settings();
            setAuthSettings(settings);
        };
        fetchAuthSettings();
    }, []);

    const ConnectRepoFormButton = ({method, onSelection}: {method: string; onSelection: (method: string) => void}) => {
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
                                const formState = formApi.current.getFormState();
                                formApi.current.setFormState({
                                    ...formState,
                                    errors: {}
                                });
                            }
                        })
                    )}
                />
            </div>
        );
    };

    const onChooseDefaultValues = (): FormValues => {
        return {type: 'git', ghType: 'GitHub', write: false};
    };

    const onValidateErrors = (params: FormValues): FormErrors => {
        switch (method) {
            case ConnectionMethod.SSH:
                const sshValues = params as NewSSHRepoParams;
                return {
                    url: !sshValues.url && 'Repository URL is required'
                };
            case ConnectionMethod.HTTPS:
                const validURLValues = params as NewHTTPSRepoParams;
                return {
                    url:
                        (!validURLValues.url && 'Repository URL is required') ||
                        (credsTemplate && !isHTTPOrHTTPSUrl(validURLValues.url) && !validURLValues.enableOCI && params.type != 'oci' && 'Not a valid HTTP/HTTPS URL') ||
                        (credsTemplate && !isOCIUrl(validURLValues.url) && params.type == 'oci' && 'Not a valid OCI URL'),
                    name: validURLValues.type === 'helm' && !validURLValues.name && 'Name is required',
                    username: !validURLValues.username && validURLValues.password && 'Username is required if password is given.',
                    password: !validURLValues.password && validURLValues.username && 'Password is required if username is given.',
                    tlsClientCertKey: !validURLValues.tlsClientCertKey && validURLValues.tlsClientCertData && 'TLS client cert key is required if TLS client cert is given.',
                    bearerToken:
                        (validURLValues.password && validURLValues.bearerToken && 'Either the password or the bearer token must be set, but not both.') ||
                        (validURLValues.bearerToken && validURLValues.type != 'git' && 'Bearer token is only supported for Git BitBucket Data Center repositories.')
                };
            case ConnectionMethod.GITHUBAPP:
                const githubAppValues = params as NewGitHubAppRepoParams;
                return {
                    url: (!githubAppValues.url && 'Repository URL is required') || (credsTemplate && !isHTTPOrHTTPSUrl(githubAppValues.url) && 'Not a valid HTTP/HTTPS URL'),
                    githubAppId: !githubAppValues.githubAppId && 'GitHub App ID is required',
                    githubAppInstallationId: !githubAppValues.githubAppInstallationId && 'GitHub App installation ID is required',
                    githubAppPrivateKey: !githubAppValues.githubAppPrivateKey && 'GitHub App private Key is required'
                };
            case ConnectionMethod.GOOGLECLOUD:
                const googleCloudValues = params as NewGoogleCloudSourceRepoParams;
                return {
                    url: (!googleCloudValues.url && 'Repo URL is required') || (credsTemplate && !isHTTPOrHTTPSUrl(googleCloudValues.url) && 'Not a valid HTTP/HTTPS URL'),
                    gcpServiceAccountKey: !googleCloudValues.gcpServiceAccountKey && 'GCP service account key is required'
                };
        }
    };

    const SlidingPanelHeader = () => {
        return (
            <>
                {showConnectRepo() && (
                    <>
                        <button
                            className='argo-button argo-button--base'
                            onClick={() => {
                                credsTemplate.current = false;
                                formApi.current.submitForm(null);
                            }}>
                            <Spinner show={connecting} style={{marginRight: '5px'}} />
                            Connect
                        </button>{' '}
                        <button
                            className='argo-button argo-button--base'
                            onClick={() => {
                                credsTemplate.current = true;
                                formApi.current.submitForm(null);
                            }}>
                            Save as credentials template
                        </button>{' '}
                        <button onClick={() => setConnectRepo(false)} className='argo-button argo-button--base-o'>
                            Cancel
                        </button>
                    </>
                )}
                {displayEditPanel && (
                    <button onClick={() => setDisplayEditPanel(false)} className='argo-button argo-button--base-o'>
                        Cancel
                    </button>
                )}
            </>
        );
    };

    const onSubmitForm = (params: FormValues) => {
        switch (method) {
            case ConnectionMethod.SSH:
                return connectSSHRepo(params as NewSSHRepoParams);
            case ConnectionMethod.HTTPS:
                params.url = params.enableOCI && params.type != 'oci' ? stripProtocol(params.url) : params.url;
                return connectHTTPSRepo(params as NewHTTPSRepoParams);
            case ConnectionMethod.GITHUBAPP:
                return connectGitHubAppRepo(params as NewGitHubAppRepoParams);
            case ConnectionMethod.GOOGLECLOUD:
                return connectGoogleCloudSourceRepo(params as NewGoogleCloudSourceRepoParams);
        }
    };

    const displayEditSliding = (repo: models.Repository) => {
        setCurrentRepo(repo);
        setDisplayEditPanel(true);
    };

    // Whether url is a http or https url
    const isHTTPOrHTTPSUrl = (url: string) => {
        if (url.match(/^https?:\/\/.*$/gi)) {
            return true;
        } else {
            return false;
        }
    };

    // Whether url is an oci url (simple version)
    const isOCIUrl = (url: string) => {
        if (url.match(/^oci:\/\/.*$/gi)) {
            return true;
        } else {
            return false;
        }
    };

    const stripProtocol = (url: string) => {
        return url.replace('https://', '').replace('oci://', '');
    };

    // only connections of git type which is not via GitHub App are updatable
    const isRepoUpdatable = (repo: models.Repository) => {
        return isHTTPOrHTTPSUrl(repo.repo) && repo.type === 'git' && !repo.githubAppId;
    };

    // Forces a reload of configured repositories, circumventing the cache
    const refreshRepoList = async (updatedRepo?: string) => {
        // Refresh the credentials template list
        credsLoader.current.reload();

        try {
            await services.repos.listNoCache();
            repoLoader.current.reload();
            ctx.notifications.show({
                content: updatedRepo ? `Successfully updated ${updatedRepo} repository` : 'Successfully reloaded list of repositories',
                type: NotificationType.Success
            });
        } catch (e) {
            ctx.notifications.show({
                content: <ErrorNotification title='Could not refresh list of repositories' e={e} />,
                type: NotificationType.Error
            });
        }
    };

    // Empty all fields in connect repository form
    const clearConnectRepoForm = () => {
        credsTemplate.current = false;
        formApi.current.resetAll();
    };

    // Connect a new repository or create a repository credentials for SSH repositories
    const connectSSHRepo = async (params: NewSSHRepoParams) => {
        if (credsTemplate.current) {
            createSSHCreds({url: params.url, sshPrivateKey: params.sshPrivateKey, write: params.write});
        } else {
            setConnecting(true);
            try {
                if (params.write) {
                    await services.repos.createSSHWrite(params);
                } else {
                    await services.repos.createSSH(params);
                }
                repoLoader.current.reload();
                setConnectRepo(false);
            } catch (e) {
                ctx.notifications.show({
                    content: <ErrorNotification title='Unable to connect SSH repository' e={e} />,
                    type: NotificationType.Error
                });
            } finally {
                setConnecting(false);
            }
        }
    };

    // Connect a new repository or create a repository credentials for HTTPS repositories
    const connectHTTPSRepo = async (params: NewHTTPSRepoParams) => {
        if (credsTemplate.current) {
            await createHTTPSCreds({
                type: params.type,
                url: params.url,
                username: params.username,
                password: params.password,
                bearerToken: params.bearerToken,
                tlsClientCertData: params.tlsClientCertData,
                tlsClientCertKey: params.tlsClientCertKey,
                proxy: params.proxy,
                noProxy: params.noProxy,
                forceHttpBasicAuth: params.forceHttpBasicAuth,
                enableOCI: params.enableOCI,
                write: params.write,
                useAzureWorkloadIdentity: params.useAzureWorkloadIdentity,
                insecureOCIForceHttp: params.insecureOCIForceHttp,
                enablePartialClone: params.enablePartialClone,
                sparsePaths: params.sparsePaths
            });
        } else {
            setConnecting(true);
            try {
                if (params.write) {
                    await services.repos.createHTTPSWrite(params);
                } else {
                    await services.repos.createHTTPS(params);
                }
                repoLoader.current.reload();
                setConnectRepo(false);
            } catch (e) {
                ctx.notifications.show({
                    content: <ErrorNotification title='Unable to connect HTTPS repository' e={e} />,
                    type: NotificationType.Error
                });
            } finally {
                setConnecting(false);
            }
        }
    };

    // Update an existing repository for HTTPS repositories
    const updateHTTPSRepo = async (params: NewHTTPSRepoParams) => {
        try {
            if (params.write) {
                await services.repos.updateHTTPSWrite(params);
            } else {
                await services.repos.updateHTTPS(params);
            }
            repoLoader.current.reload();
            setDisplayEditPanel(false);
            refreshRepoList(params.url);
        } catch (e) {
            ctx.notifications.show({
                content: <ErrorNotification title='Unable to update HTTPS repository' e={e} />,
                type: NotificationType.Error
            });
        } finally {
            setConnecting(false);
        }
    };

    // Connect a new repository or create a repository credentials for GitHub App repositories
    const connectGitHubAppRepo = async (params: NewGitHubAppRepoParams) => {
        if (credsTemplate.current) {
            createGitHubAppCreds({
                url: params.url,
                githubAppPrivateKey: params.githubAppPrivateKey,
                githubAppId: params.githubAppId,
                githubAppInstallationId: params.githubAppInstallationId,
                githubAppEnterpriseBaseURL: params.githubAppEnterpriseBaseURL,
                tlsClientCertData: params.tlsClientCertData,
                tlsClientCertKey: params.tlsClientCertKey,
                proxy: params.proxy,
                noProxy: params.noProxy,
                write: params.write
            });
        } else {
            setConnecting(true);
            try {
                if (params.write) {
                    await services.repos.createGitHubAppWrite(params);
                } else {
                    await services.repos.createGitHubApp(params);
                }
                repoLoader.current.reload();
                setConnectRepo(false);
            } catch (e) {
                ctx.notifications.show({
                    content: <ErrorNotification title='Unable to connect GitHub App repository' e={e} />,
                    type: NotificationType.Error
                });
            } finally {
                setConnecting(false);
            }
        }
    };

    // Connect a new repository or create a repository credentials for GitHub App repositories
    const connectGoogleCloudSourceRepo = async (params: NewGoogleCloudSourceRepoParams) => {
        if (credsTemplate.current) {
            createGoogleCloudSourceCreds({
                url: params.url,
                gcpServiceAccountKey: params.gcpServiceAccountKey,
                write: params.write
            });
        } else {
            setConnecting(true);
            try {
                if (params.write) {
                    await services.repos.createGoogleCloudSourceWrite(params);
                } else {
                    await services.repos.createGoogleCloudSource(params);
                }
                repoLoader.current.reload();
                setConnectRepo(false);
            } catch (e) {
                ctx.notifications.show({
                    content: <ErrorNotification title='Unable to connect Google Cloud Source repository' e={e} />,
                    type: NotificationType.Error
                });
            } finally {
                setConnecting(false);
            }
        }
    };

    const createHTTPSCreds = async (params: NewHTTPSRepoCredsParams) => {
        try {
            if (params.write) {
                await services.repocreds.createHTTPSWrite(params);
            } else {
                await services.repocreds.createHTTPS(params);
            }
            credsLoader.current.reload();
            setConnectRepo(false);
        } catch (e) {
            ctx.notifications.show({
                content: <ErrorNotification title='Unable to create HTTPS credentials' e={e} />,
                type: NotificationType.Error
            });
        }
    };

    const createSSHCreds = async (params: NewSSHRepoCredsParams) => {
        try {
            if (params.write) {
                await services.repocreds.createSSHWrite(params);
            } else {
                await services.repocreds.createSSH(params);
            }
            credsLoader.current.reload();
            setConnectRepo(false);
        } catch (e) {
            ctx.notifications.show({
                content: <ErrorNotification title='Unable to create SSH credentials' e={e} />,
                type: NotificationType.Error
            });
        }
    };

    const createGitHubAppCreds = async (params: NewGitHubAppRepoCredsParams) => {
        try {
            if (params.write) {
                await services.repocreds.createGitHubAppWrite(params);
            } else {
                await services.repocreds.createGitHubApp(params);
            }
            credsLoader.current.reload();
            setConnectRepo(false);
        } catch (e) {
            ctx.notifications.show({
                content: <ErrorNotification title='Unable to create GitHub App credentials' e={e} />,
                type: NotificationType.Error
            });
        }
    };

    const createGoogleCloudSourceCreds = async (params: NewGoogleCloudSourceRepoCredsParams) => {
        try {
            if (params.write) {
                await services.repocreds.createGoogleCloudSourceWrite(params);
            } else {
                await services.repocreds.createGoogleCloudSource(params);
            }
            credsLoader.current.reload();
            setConnectRepo(false);
        } catch (e) {
            ctx.notifications.show({
                content: <ErrorNotification title='Unable to create Google Cloud Source credentials' e={e} />,
                type: NotificationType.Error
            });
        }
    };

    // Remove a repository from the configuration
    const disconnectRepo = async (repo: string, project: string, write: boolean) => {
        const confirmed = await ctx.popup.confirm('Disconnect repository', `Are you sure you want to disconnect '${repo}'?`);
        if (confirmed) {
            try {
                if (write) {
                    await services.repos.deleteWrite(repo, project || '');
                } else {
                    await services.repos.delete(repo, project || '');
                }
                repoLoader.current.reload();
            } catch (e) {
                ctx.notifications.show({
                    content: <ErrorNotification title='Unable to disconnect repository' e={e} />,
                    type: NotificationType.Error
                });
            }
        }
    };

    // Remove repository credentials from the configuration
    const removeRepoCreds = async (url: string, write: boolean) => {
        const confirmed = await ctx.popup.confirm('Remove repository credentials', `Are you sure you want to remove credentials for URL prefix '${url}'?`);
        if (confirmed) {
            try {
                if (write) {
                    await services.repocreds.deleteWrite(url);
                } else {
                    await services.repocreds.delete(url);
                }
                credsLoader.current.reload();
            } catch (e) {
                ctx.notifications.show({
                    content: <ErrorNotification title='Unable to remove repository credentials' e={e} />,
                    type: NotificationType.Error
                });
            }
        }
    };

    // filtering function
    const filterRepos = (repos: models.Repository[], type: string, project: string, status: string, name: string) => {
        let newRepos = repos;

        if (name && name.trim() !== '') {
            const response = filteredName(newRepos, name);
            newRepos = response;
        }

        if (type !== 'all') {
            const response = filteredType(newRepos, type);
            newRepos = response;
        }

        if (status !== 'all') {
            const response = filteredStatus(newRepos, status);
            newRepos = response;
        }

        if (project !== 'all') {
            const response = filteredProject(newRepos, project);
            newRepos = response;
        }

        return newRepos;
    };

    const filteredName = (repos: models.Repository[], name: string) => {
        const trimmedName = name.trim();
        if (trimmedName === '') {
            return repos;
        }
        const newRepos = repos.filter(
            repo => (repo.name && repo.name.toLowerCase().includes(trimmedName.toLowerCase())) || repo.repo.toLowerCase().includes(trimmedName.toLowerCase())
        );
        return newRepos;
    };

    const filteredStatus = (repos: models.Repository[], status: string) => {
        const newRepos = repos.filter(repo => repo.connectionState.status.includes(status));
        return newRepos;
    };

    const filteredProject = (repos: models.Repository[], project: string) => {
        const newRepos = repos.filter(repo => repo.project && repo.project.includes(project));
        return newRepos;
    };

    const filteredType = (repos: models.Repository[], type: string) => {
        const newRepos = repos.filter(repo => repo.type.includes(type));
        return newRepos;
    };

    // Whether to show the new repository connection dialogue on the page
    const showConnectRepo = (): boolean => {
        return new URLSearchParams(location.search).get('addRepo') === 'true';
    };

    const setConnectRepo = (val: boolean) => {
        clearConnectRepoForm();
        ctx.history.push({
            pathname: match.url,
            search: `?addRepo=${val}`
        });
    };

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
                            action: () => setConnectRepo(true)
                        },
                        {
                            iconClassName: 'fa fa-redo',
                            title: 'Refresh list',
                            action: () => {
                                refreshRepoList();
                            }
                        }
                    ]
                }
            }}>
            <div className='repos-list'>
                <div className='argo-container'>
                    <div style={{display: 'flex', margin: '20px 0', justifyContent: 'space-between'}}>
                        <div style={{display: 'flex', gap: '8px', width: '50%'}}>
                            <DropDownMenu
                                items={[
                                    {
                                        title: 'all',
                                        action: () => setTypeProperty('all')
                                    },
                                    {
                                        title: 'git',
                                        action: () => setTypeProperty('git')
                                    },
                                    {
                                        title: 'helm',
                                        action: () => setTypeProperty('helm')
                                    }
                                ]}
                                anchor={() => (
                                    <>
                                        <a style={{whiteSpace: 'nowrap'}}>
                                            Type: {typeProperty} <i className='fa fa-caret-down' />
                                        </a>
                                        &nbsp;
                                    </>
                                )}
                                qeId='type-menu'
                            />
                            <DataLoader load={services.repos.list} ref={loader => (repoLoader.current = loader)}>
                                {(repos: models.Repository[]) => {
                                    const projectValues = Array.from(new Set(repos.map(repo => repo.project)));

                                    const projectItems = [
                                        {
                                            title: 'all',
                                            action: () => setProjectProperty('all')
                                        },
                                        ...projectValues
                                            .filter(project => project && project.trim() !== '')
                                            .map(project => ({
                                                title: project,
                                                action: () => setProjectProperty(project)
                                            }))
                                    ];

                                    return (
                                        <DropDownMenu
                                            items={projectItems}
                                            anchor={() => (
                                                <>
                                                    <a style={{whiteSpace: 'nowrap'}}>
                                                        Project: {projectProperty} <i className='fa fa-caret-down' />
                                                    </a>
                                                    &nbsp;
                                                </>
                                            )}
                                            qeId='project-menu'
                                        />
                                    );
                                }}
                            </DataLoader>
                            <DropDownMenu
                                items={[
                                    {
                                        title: 'all',
                                        action: () => setStatusProperty('all')
                                    },
                                    {
                                        title: 'Successful',
                                        action: () => setStatusProperty('Successful')
                                    },
                                    {
                                        title: 'Failed',
                                        action: () => setStatusProperty('Failed')
                                    },
                                    {
                                        title: 'Unknown',
                                        action: () => setStatusProperty('Unknown')
                                    }
                                ]}
                                anchor={() => (
                                    <>
                                        <a style={{whiteSpace: 'nowrap'}}>
                                            Status: {statusProperty} <i className='fa fa-caret-down' />
                                        </a>
                                        &nbsp;
                                    </>
                                )}
                                qeId='status-menu'
                            />
                        </div>
                        <div className='search-bar' style={{display: 'flex', alignItems: 'flex-end', width: '100%'}}></div>
                        <input type='text' className='argo-field' placeholder='Search Name' value={name} onChange={e => setName(e.target.value)} />
                    </div>
                    <DataLoader load={services.repos.list} ref={loader => (repoLoader.current = loader)}>
                        {(repos: models.Repository[]) => {
                            const filteredRepos = filterRepos(repos, typeProperty, projectProperty, statusProperty, name);

                            return (
                                (filteredRepos.length > 0 && (
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
                                        {filteredRepos.map(repo => (
                                            <div
                                                className={`argo-table-list__row ${isRepoUpdatable(repo) ? 'item-clickable' : ''}`}
                                                key={repo.repo}
                                                onClick={() => (isRepoUpdatable(repo) ? displayEditSliding(repo) : null)}>
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
                                                                        ctx.navigation.goto('/applications', {
                                                                            new: JSON.stringify({spec: {source: {repoURL: repo.repo}}})
                                                                        })
                                                                },
                                                                {
                                                                    title: 'Disconnect',
                                                                    action: () => disconnectRepo(repo.repo, repo.project, false)
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
                            );
                        }}
                    </DataLoader>
                </div>
                <div className='argo-container'>
                    <DataLoader load={() => services.repocreds.list()} ref={loader => (credsLoader.current = loader)}>
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
                                                        items={[
                                                            {
                                                                title: 'Remove',
                                                                action: () => removeRepoCreds(repo.url, false)
                                                            }
                                                        ]}
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
                {authSettings?.hydratorEnabled && (
                    <div className='argo-container'>
                        <DataLoader load={() => services.repos.listWrite()} ref={loader => (repoLoader.current = loader)}>
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
                                                className={`argo-table-list__row ${isRepoUpdatable(repo) ? 'item-clickable' : ''}`}
                                                key={repo.repo}
                                                onClick={() => (isRepoUpdatable(repo) ? displayEditSliding(repo) : null)}>
                                                <div className='row'>
                                                    <div className='columns small-1'>
                                                        <i className='icon argo-icon-git' />
                                                    </div>
                                                    <div className='columns small-1'>write</div>
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
                                                                        ctx.navigation.goto('/applications', {
                                                                            new: JSON.stringify({spec: {sourceHydrator: {drySource: {repoURL: repo.repo}}}})
                                                                        })
                                                                },
                                                                {
                                                                    title: 'Disconnect',
                                                                    action: () => disconnectRepo(repo.repo, repo.project, true)
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
                )}
                {authSettings?.hydratorEnabled && (
                    <div className='argo-container'>
                        <DataLoader load={() => services.repocreds.listWrite()} ref={loader => (credsLoader.current = loader)}>
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
                                                            items={[
                                                                {
                                                                    title: 'Remove',
                                                                    action: () => removeRepoCreds(repo.url, true)
                                                                }
                                                            ]}
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
                )}
            </div>
            <SlidingPanel
                isShown={showConnectRepo() || displayEditPanel}
                onClose={() => {
                    if (!displayEditPanel && showConnectRepo()) {
                        setConnectRepo(false);
                    }
                    if (displayEditPanel) {
                        setDisplayEditPanel(false);
                    }
                }}
                header={<SlidingPanelHeader />}>
                {showConnectRepo() && <ConnectRepoFormButton method={method} onSelection={setMethod} />}
                {displayEditPanel && <RepoDetails repo={currentRepo} save={(params: NewHTTPSRepoParams) => updateHTTPSRepo(params)} />}
                {!displayEditPanel && (
                    <DataLoader load={() => services.projects.list('items.metadata.name').then(projects => projects.map(proj => proj.metadata.name).sort())}>
                        {projects => (
                            <Form
                                onSubmit={onSubmitForm}
                                getApi={api => (formApi.current = api)}
                                defaultValues={onChooseDefaultValues()}
                                validateError={(values: FormValues) => onValidateErrors(values)}>
                                {formApi => (
                                    <form onSubmit={formApi.submitForm} role='form' className='repos-list width-control'>
                                        {authSettings?.hydratorEnabled && (
                                            <div className='white-box'>
                                                <p>SAVE AS WRITE CREDENTIAL (ALPHA)</p>
                                                <p>
                                                    The Source Hydrator is an Alpha feature which enables Applications to push hydrated manifests to git before syncing. To use
                                                    Source Hydrator for a repository, you must save two credentials: a read credential for pulling manifests and a write credential
                                                    for pushing hydrated manifests. If you add a write credential for a repository, then{' '}
                                                    <strong>any Application that can sync from the repo can also push hydrated manifests to that repo.</strong> Do not use this
                                                    feature until you've read its documentation and understand the security implications.
                                                </p>
                                                <div className='argo-form-row'>
                                                    <FormField formApi={formApi} label='Save as write credential' field='write' component={CheckboxField} />
                                                </div>
                                            </div>
                                        )}
                                        {method === ConnectionMethod.SSH && (
                                            <div className='white-box'>
                                                <p>CONNECT REPO USING SSH</p>
                                                {formApi.getFormState().values.write === false && (
                                                    <div className='argo-form-row'>
                                                        <FormField formApi={formApi} label='Name (mandatory for Helm)' field='name' component={Text} />
                                                    </div>
                                                )}
                                                {formApi.getFormState().values.write === false && (
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label='Project'
                                                            field='project'
                                                            component={AutocompleteField}
                                                            componentProps={{items: projects}}
                                                        />
                                                    </div>
                                                )}
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
                                                {formApi.getFormState().values.write === false && (
                                                    <div className='argo-form-row'>
                                                        <FormField formApi={formApi} label='Enable LFS support (Git only)' field='enableLfs' component={CheckboxField} />
                                                        <HelpIcon title='This setting is ignored when creating as credential template.' />
                                                    </div>
                                                )}
                                                <div className='argo-form-row'>
                                                    <FormField formApi={formApi} label='Proxy (optional)' field='proxy' component={Text} />
                                                </div>
                                                <div className='argo-form-row'>
                                                    <FormField formApi={formApi} label='NoProxy (optional)' field='noProxy' component={Text} />
                                                </div>
                                                {formApi.getFormState().values.write === false && (
                                                    <>
                                                        <div className='argo-form-row'>
                                                            <FormField formApi={formApi} label='Enable partial clone' field='enablePartialClone' component={CheckboxField} />
                                                            <HelpIcon title='Enable Git partial clone to download only necessary objects. This can significantly reduce clone time and size for large repositories.' />
                                                        </div>
                                                        <div className='argo-form-row'>
                                                            <FormField formApi={formApi} label='Sparse paths (optional, comma-separated)' field='sparsePaths' component={Text} />
                                                            <HelpIcon title='Specify paths for sparse checkout (comma-separated). Only specified paths will be checked out.' />
                                                        </div>
                                                    </>
                                                )}
                                            </div>
                                        )}
                                        {method === ConnectionMethod.HTTPS && (
                                            <div className='white-box'>
                                                <p>CONNECT REPO USING HTTP/HTTPS</p>
                                                <div className='argo-form-row'>
                                                    <FormField
                                                        formApi={formApi}
                                                        label='Type'
                                                        field='type'
                                                        component={FormSelect}
                                                        componentProps={{options: ['git', 'helm', 'oci']}}
                                                    />
                                                </div>
                                                {(formApi.getFormState().values.type === 'helm' || formApi.getFormState().values.type === 'git') && (
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label={`Name ${formApi.getFormState().values.type === 'git' ? '(optional)' : ''}`}
                                                            field='name'
                                                            component={Text}
                                                        />
                                                    </div>
                                                )}
                                                {formApi.getFormState().values.write === false && (
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label='Project'
                                                            field='project'
                                                            component={AutocompleteField}
                                                            componentProps={{items: projects}}
                                                        />
                                                    </div>
                                                )}
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
                                                {formApi.getFormState().values.type === 'git' && (
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label='Bearer token (optional, for BitBucket Data Center only)'
                                                            field='bearerToken'
                                                            component={Text}
                                                            componentProps={{type: 'password'}}
                                                        />
                                                    </div>
                                                )}
                                                <div className='argo-form-row'>
                                                    <FormField formApi={formApi} label='TLS client certificate (optional)' field='tlsClientCertData' component={TextArea} />
                                                </div>
                                                <div className='argo-form-row'>
                                                    <FormField formApi={formApi} label='TLS client certificate key (optional)' field='tlsClientCertKey' component={TextArea} />
                                                </div>
                                                <div className='argo-form-row'>
                                                    <FormField formApi={formApi} label='Skip server verification' field='insecure' component={CheckboxField} />
                                                    <HelpIcon title='This setting is ignored when creating as credential template.' />
                                                </div>
                                                {formApi.getFormState().values.type === 'git' && (
                                                    <>
                                                        <div className='argo-form-row'>
                                                            <FormField formApi={formApi} label='Force HTTP basic auth' field='forceHttpBasicAuth' component={CheckboxField} />
                                                        </div>
                                                        <div className='argo-form-row'>
                                                            <FormField formApi={formApi} label='Enable LFS support (Git only)' field='enableLfs' component={CheckboxField} />
                                                            <HelpIcon title='This setting is ignored when creating as credential template.' />
                                                        </div>
                                                    </>
                                                )}
                                                <div className='argo-form-row'>
                                                    <FormField formApi={formApi} label='Proxy (optional)' field='proxy' component={Text} />
                                                </div>
                                                <div className='argo-form-row'>
                                                    <FormField formApi={formApi} label='NoProxy (optional)' field='noProxy' component={Text} />
                                                </div>
                                                <div className='argo-form-row'>
                                                    {formApi.getFormState().values.type !== 'oci' ? (
                                                        <FormField formApi={formApi} label='Enable OCI' field='enableOCI' component={CheckboxField} />
                                                    ) : (
                                                        <FormField formApi={formApi} label='Insecure HTTP Only' field='insecureOCIForceHttp' component={CheckboxField} />
                                                    )}
                                                </div>
                                                <div className='argo-form-row'>
                                                    <FormField formApi={formApi} label='Use Azure Workload Identity' field='useAzureWorkloadIdentity' component={CheckboxField} />
                                                </div>
                                                {formApi.getFormState().values.type === 'git' && formApi.getFormState().values.write === false && (
                                                    <>
                                                        <div className='argo-form-row'>
                                                            <FormField formApi={formApi} label='Enable partial clone' field='enablePartialClone' component={CheckboxField} />
                                                            <HelpIcon title='Enable Git partial clone to download only necessary objects. This can significantly reduce clone time and size for large repositories.' />
                                                        </div>
                                                        <div className='argo-form-row'>
                                                            <FormField formApi={formApi} label='Sparse paths (optional, comma-separated)' field='sparsePaths' component={Text} />
                                                            <HelpIcon title='Specify paths for sparse checkout (comma-separated). Only specified paths will be checked out.' />
                                                        </div>
                                                    </>
                                                )}
                                            </div>
                                        )}
                                        {method === ConnectionMethod.GITHUBAPP && (
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
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label='GitHub Enterprise Base URL (e.g. https://ghe.example.com/formApi/v3)'
                                                            field='githubAppEnterpriseBaseURL'
                                                            component={Text}
                                                        />
                                                    </div>
                                                )}
                                                <div className='argo-form-row'>
                                                    <FormField formApi={formApi} label='Project' field='project' component={AutocompleteField} componentProps={{items: projects}} />
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
                                                    <>
                                                        <div className='argo-form-row'>
                                                            <FormField formApi={formApi} label='TLS client certificate (optional)' field='tlsClientCertData' component={TextArea} />
                                                        </div>
                                                        <div className='argo-form-row'>
                                                            <FormField
                                                                formApi={formApi}
                                                                label='TLS client certificate key (optional)'
                                                                field='tlsClientCertKey'
                                                                component={TextArea}
                                                            />
                                                        </div>
                                                    </>
                                                )}
                                                <div className='argo-form-row'>
                                                    <FormField formApi={formApi} label='Proxy (optional)' field='proxy' component={Text} />
                                                </div>
                                                <div className='argo-form-row'>
                                                    <FormField formApi={formApi} label='NoProxy (optional)' field='noProxy' component={Text} />
                                                </div>
                                                {formApi.getFormState().values.write === false && (
                                                    <>
                                                        <div className='argo-form-row'>
                                                            <FormField formApi={formApi} label='Enable partial clone' field='enablePartialClone' component={CheckboxField} />
                                                            <HelpIcon title='Enable Git partial clone to download only necessary objects. This can significantly reduce clone time and size for large repositories.' />
                                                        </div>
                                                        <div className='argo-form-row'>
                                                            <FormField formApi={formApi} label='Sparse paths (comma-separated)' field='sparsePaths' component={Text} />
                                                            <HelpIcon title='Specify paths for sparse checkout (comma-separated). Only specified paths will be checked out.' />
                                                        </div>
                                                    </>
                                                )}
                                            </div>
                                        )}
                                        {method === ConnectionMethod.GOOGLECLOUD && (
                                            <div className='white-box'>
                                                <p>CONNECT REPO USING GOOGLE CLOUD</p>
                                                <div className='argo-form-row'>
                                                    <FormField formApi={formApi} label='Project' field='project' component={AutocompleteField} componentProps={{items: projects}} />
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
};

export default withRouter(ReposList);
