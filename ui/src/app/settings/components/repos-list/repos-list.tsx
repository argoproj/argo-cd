import {AutocompleteField, DropDownMenu, FormField, FormSelect, HelpIcon, NotificationType, SlidingPanel, Tooltip} from 'argo-ui';
import * as React from 'react';
import * as ReactDOM from 'react-dom';
import {withRouter, RouteComponentProps} from 'react-router-dom';
import {Form, FormValues, FormApi, Text, TextArea, FormErrors} from 'argo-ui';
import {Context} from '../../../shared/context';
import {
    ActionMenu,
    CheckboxField,
    ConnectionStateIcon,
    DataLoader,
    EmptyState,
    ErrorNotification,
    IconColumn,
    NumberField,
    Page,
    Paginate,
    Repo,
    SearchBar,
    Spinner
} from '../../../shared/components';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {RepoDetails} from '../repo-details/repo-details';
import {useQuery} from '../../../shared/hooks/query';
import {useListSort} from '../../../shared/hooks/use-list-sort';
import {FlexTopBar} from '../../../shared/components';
import {useSidebarTarget} from '../../../sidebar/sidebar';
import {
    filterRepos,
    getRepoFilterResults,
    ReposFilter,
    ReposListPreferences,
    ReposListPreferencesHelper,
    UnifiedRepo,
    getRepoUrl,
    getRepoName,
    getRepoType,
    getRepoProject,
    getConnectionState,
    isWrite,
    isTemplate
} from './repos-filter';

// Helper functions to convert to UnifiedRepo
const repoToUnified = (repo: models.Repository, isWriteFlag: boolean): UnifiedRepo => (isWriteFlag ? {writeRepo: repo} : {readRepo: repo});

const credToUnified = (cred: models.RepoCreds, isWriteFlag: boolean): UnifiedRepo => (isWriteFlag ? {writeCred: cred} : {readCred: cred});

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
    depth?: number;
    // write should be true if saving as a write credential.
    write: boolean;
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
    depth?: number;
    // write should be true if saving as a write credential.
    write: boolean;
    useAzureWorkloadIdentity: boolean;
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
    depth?: number;
    // write should be true if saving as a write credential.
    write: boolean;
}

interface NewGoogleCloudSourceRepoParams {
    type: string;
    name: string;
    url: string;
    gcpServiceAccountKey: string;
    proxy: string;
    noProxy: string;
    project?: string;
    depth?: number;
    // write should be true if saving as a write credential.
    write: boolean;
}

interface NewAzureServicePrincipalRepoParams {
    type: string;
    name: string;
    url: string;
    azureServicePrincipalClientId: string;
    azureServicePrincipalClientSecret: string;
    azureServicePrincipalTenantId: string;
    azureActiveDirectoryEndpoint: string;
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

interface NewAzureServicePrincipalRepoCredsParams {
    url: string;
    azureServicePrincipalClientId: string;
    azureServicePrincipalClientSecret: string;
    azureServicePrincipalTenantId: string;
    azureActiveDirectoryEndpoint: string;
    proxy: string;
    noProxy: string;
    project?: string;
    // write should be true if saving as a write credential.
    write: boolean;
}

export enum ConnectionMethod {
    SSH = 'via SSH',
    HTTPS = 'via HTTP/HTTPS',
    GITHUBAPP = 'via GitHub App',
    GOOGLECLOUD = 'via Google Cloud',
    AZURESERVICEPRINCIPAL = 'via Azure Service Principal'
}

const ConnectRepoFormButton = ({method, onSelection, formApi}: {method: string; onSelection: (method: string) => void; formApi: React.MutableRefObject<FormApi | null>}) => {
    return (
        <div className='white-box'>
            <p>Choose your connection method:</p>
            <DropDownMenu
                anchor={() => (
                    <p>
                        {method.toUpperCase()} <i className='fa fa-caret-down' />
                    </p>
                )}
                items={[ConnectionMethod.SSH, ConnectionMethod.HTTPS, ConnectionMethod.GITHUBAPP, ConnectionMethod.GOOGLECLOUD, ConnectionMethod.AZURESERVICEPRINCIPAL].map(
                    (
                        connectMethod:
                            ConnectionMethod.SSH | ConnectionMethod.HTTPS | ConnectionMethod.GITHUBAPP | ConnectionMethod.GOOGLECLOUD | ConnectionMethod.AZURESERVICEPRINCIPAL
                    ) => ({
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

const SlidingPanelHeader = ({
    showConnectRepo,
    displayEditPanel,
    connecting,
    onConnect,
    onSaveCredsTemplate,
    setConnectRepo,
    setDisplayEditPanel
}: {
    showConnectRepo: boolean;
    displayEditPanel: boolean;
    connecting: boolean;
    onConnect: () => void;
    onSaveCredsTemplate: () => void;
    setConnectRepo: (val: boolean) => void;
    setDisplayEditPanel: (val: boolean) => void;
}) => {
    return (
        <>
            {showConnectRepo && (
                <>
                    <button className='argo-button argo-button--base' onClick={onConnect}>
                        <Spinner show={connecting} style={{marginRight: '5px'}} />
                        Connect
                    </button>{' '}
                    <button className='argo-button argo-button--base' onClick={onSaveCredsTemplate}>
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

export const ReposList = ({match, location}: RouteComponentProps) => {
    const [connecting, setConnecting] = React.useState(false);
    const [method, setMethod] = React.useState<string>(ConnectionMethod.SSH);
    const [currentRepo, setCurrentRepo] = React.useState<UnifiedRepo | null>(null);
    const [displayEditPanel, setDisplayEditPanel] = React.useState(false);
    const [authSettings, setAuthSettings] = React.useState<models.AuthSettings | null>(null);
    const query = useQuery();
    const name = query.get('search') || '';
    const [filterPref, setFilterPref] = React.useState<ReposListPreferences>({
        typeFilter: query.getAll('type') || [],
        projectFilter: query.getAll('project') || [],
        statusFilter: query.getAll('status') || [],
        credentialTypeFilter: query.getAll('permission') || [],
        templateFilter: query.getAll('category') || []
    });
    const [page, setPage] = React.useState(0);
    const sidebarTarget = useSidebarTarget();

    const ctx = React.useContext(Context);

    const filterQuery = (pref: ReposListPreferences, search: string) => ({
        type: pref.typeFilter.length > 0 ? pref.typeFilter : null,
        project: pref.projectFilter.length > 0 ? pref.projectFilter : null,
        status: pref.statusFilter.length > 0 ? pref.statusFilter : null,
        permission: pref.credentialTypeFilter.length > 0 ? pref.credentialTypeFilter : null,
        category: pref.templateFilter.length > 0 ? pref.templateFilter : null,
        search: search || null
    });

    const onFilterChange = (newPref: ReposListPreferences) => {
        setFilterPref(newPref);
        ctx.navigation.goto('.', filterQuery(newPref, name), {replace: true});
        setPage(0);
    };

    type SortKey = 'type' | 'name' | 'repository' | 'project' | 'permission';
    const {sortKey, requestSort, sortIcon, compareString, compareNumber} = useListSort<SortKey>('name');

    const formApi = React.useRef<FormApi | null>(null);
    const credsTemplate = React.useRef<boolean>(false);
    const repoLoader = React.useRef<DataLoader | null>(null);

    React.useEffect(() => {
        const fetchAuthSettings = async () => {
            const settings = await services.authService.settings();
            setAuthSettings(settings);
        };
        fetchAuthSettings();
    }, []);

    const onChooseDefaultValues = (): FormValues => {
        return {type: 'git', ghType: 'GitHub', azureType: 'Azure Public Cloud', write: false};
    };

    const onValidateErrors = (params: FormValues): FormErrors => {
        switch (method) {
            case ConnectionMethod.SSH: {
                const sshValues = params as NewSSHRepoParams;
                return {
                    url: !sshValues.url && 'Repository URL is required',
                    depth: sshValues.depth != undefined && sshValues.depth < 0 && 'Depth must be a non-negative number'
                };
            }
            case ConnectionMethod.HTTPS: {
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
                        (validURLValues.bearerToken && validURLValues.type != 'git' && 'Bearer token is only supported for Git BitBucket Data Center repositories.'),
                    depth: validURLValues.depth != undefined && validURLValues.depth < 0 && 'Depth must be a non-negative number'
                };
            }
            case ConnectionMethod.GITHUBAPP: {
                const githubAppValues = params as NewGitHubAppRepoParams;
                return {
                    url: (!githubAppValues.url && 'Repository URL is required') || (credsTemplate && !isHTTPOrHTTPSUrl(githubAppValues.url) && 'Not a valid HTTP/HTTPS URL'),
                    githubAppId: !githubAppValues.githubAppId && 'GitHub App ID is required',
                    githubAppPrivateKey: !githubAppValues.githubAppPrivateKey && 'GitHub App private Key is required',
                    depth: githubAppValues.depth != undefined && githubAppValues.depth < 0 && 'Depth must be a non-negative number'
                };
            }
            case ConnectionMethod.GOOGLECLOUD: {
                const googleCloudValues = params as NewGoogleCloudSourceRepoParams;
                return {
                    url: (!googleCloudValues.url && 'Repo URL is required') || (credsTemplate && !isHTTPOrHTTPSUrl(googleCloudValues.url) && 'Not a valid HTTP/HTTPS URL'),
                    gcpServiceAccountKey: !googleCloudValues.gcpServiceAccountKey && 'GCP service account key is required',
                    depth: googleCloudValues.depth != undefined && googleCloudValues.depth < 0 && 'Depth must be a non-negative number'
                };
            }
            case ConnectionMethod.AZURESERVICEPRINCIPAL: {
                const azureServicePrincipalValues = params as NewAzureServicePrincipalRepoParams;
                return {
                    url:
                        (!azureServicePrincipalValues.url && 'Repository URL is required') ||
                        (credsTemplate && !isHTTPOrHTTPSUrl(azureServicePrincipalValues.url) && 'Not a valid HTTP/HTTPS URL'),
                    azureServicePrincipalClientId: !azureServicePrincipalValues.azureServicePrincipalClientId && 'Azure Service Principal Client ID is required',
                    azureServicePrincipalClientSecret: !azureServicePrincipalValues.azureServicePrincipalClientSecret && 'Azure Service Principal Client Secret is required',
                    azureServicePrincipalTenantId: !azureServicePrincipalValues.azureServicePrincipalTenantId && 'Azure Service Principal Tenant ID is required'
                };
            }
        }
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
            case ConnectionMethod.AZURESERVICEPRINCIPAL:
                return connectAzureServicePrincipalRepo(params as NewAzureServicePrincipalRepoParams);
        }
    };

    const displayEditSliding = (repo: UnifiedRepo) => {
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

    // only connections of git type which are not via GitHub App or Azure Service Principal are updatable
    const isRepoUpdatable = (item: UnifiedRepo) => {
        // Only readRepo or writeRepo can be updated (not templates)
        const repo = item.readRepo || item.writeRepo;
        if (!repo || isTemplate(item)) {
            return false;
        }
        // Check if it's an updatable repository (HTTP/HTTPS git repo without GitHub App or Azure SP)
        return isHTTPOrHTTPSUrl(repo.repo) && getRepoType(item) === 'git' && !repo.githubAppID && !repo.azureServicePrincipalClientId;
    };

    // Forces a reload of configured repositories, circumventing the cache
    const refreshRepoList = async (updatedRepo?: string) => {
        try {
            await services.repos.listNoCache();
            repoLoader.current?.reload();
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
                insecureOCIForceHttp: params.insecureOCIForceHttp
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

    // Connect a new repository or create a repository credentials for Google Cloud Source repositories
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

    // Connect a new repository or create a repository credentials for Azure Service Principal repositories
    const connectAzureServicePrincipalRepo = async (params: NewAzureServicePrincipalRepoParams) => {
        if (credsTemplate.current) {
            createAzureServicePrincipalCreds({
                url: params.url,
                azureServicePrincipalClientId: params.azureServicePrincipalClientId,
                azureServicePrincipalClientSecret: params.azureServicePrincipalClientSecret,
                azureServicePrincipalTenantId: params.azureServicePrincipalTenantId,
                azureActiveDirectoryEndpoint: params.azureActiveDirectoryEndpoint,
                proxy: params.proxy,
                noProxy: params.noProxy,
                write: params.write
            });
        } else {
            setConnecting(true);
            try {
                if (params.write) {
                    await services.repos.createAzureServicePrincipalWrite(params);
                } else {
                    await services.repos.createAzureServicePrincipal(params);
                }
                repoLoader.current.reload();
                setConnectRepo(false);
            } catch (e) {
                ctx.notifications.show({
                    content: <ErrorNotification title='Unable to connect Azure Service Principal repository' e={e} />,
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
            repoLoader.current?.reload();
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
            repoLoader.current?.reload();
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
            repoLoader.current?.reload();
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
            repoLoader.current?.reload();
            setConnectRepo(false);
        } catch (e) {
            ctx.notifications.show({
                content: <ErrorNotification title='Unable to create Google Cloud Source credentials' e={e} />,
                type: NotificationType.Error
            });
        }
    };

    const createAzureServicePrincipalCreds = async (params: NewAzureServicePrincipalRepoCredsParams) => {
        try {
            if (params.write) {
                await services.repocreds.createAzureServicePrincipalWrite(params);
            } else {
                await services.repocreds.createAzureServicePrincipal(params);
            }
            repoLoader.current?.reload();
            setConnectRepo(false);
        } catch (e) {
            ctx.notifications.show({
                content: <ErrorNotification title='Unable to create Azure Service Principal credentials' e={e} />,
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
                repoLoader.current?.reload();
            } catch (e) {
                ctx.notifications.show({
                    content: <ErrorNotification title='Unable to remove repository credentials' e={e} />,
                    type: NotificationType.Error
                });
            }
        }
    };

    const filteredName = (repos: UnifiedRepo[], search: string) => {
        const trimmedName = search.trim();
        if (trimmedName === '') {
            return repos;
        }
        return repos.filter(repo => {
            const repoUrl = getRepoUrl(repo);
            const repoName = getRepoName(repo);
            return (repoName && repoName.toLowerCase().includes(trimmedName.toLowerCase())) || repoUrl.toLowerCase().includes(trimmedName.toLowerCase());
        });
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
        <Page title='Repositories' toolbar={{breadcrumbs: [{title: 'Settings', path: '/settings'}, {title: 'Repositories'}]}}>
            <FlexTopBar
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
                    },
                    tools: (
                        <SearchBar
                            value={name}
                            onChange={value => {
                                ctx.navigation.goto('.', filterQuery(filterPref, value), {replace: true});
                                setPage(0);
                            }}
                            placeholder='Search repositories...'
                        />
                    )
                }}
            />
            <div className='repos-list'>
                <div className='argo-container'>
                    {authSettings && (
                        <DataLoader
                            load={async () => {
                                const [readRepos, readCreds, writeRepos, writeCreds] = await Promise.all([
                                    services.repos.list(),
                                    services.repocreds.list(),
                                    authSettings?.hydratorEnabled ? services.repos.listWrite() : Promise.resolve([]),
                                    authSettings?.hydratorEnabled ? services.repocreds.listWrite() : Promise.resolve([])
                                ]);

                                // Convert all to UnifiedRepo
                                const unified: UnifiedRepo[] = [
                                    ...readRepos.map(r => repoToUnified(r, false)),
                                    ...readCreds.map(c => credToUnified(c, false)),
                                    ...writeRepos.map(r => repoToUnified(r, true)),
                                    ...writeCreds.map(c => credToUnified(c, true))
                                ];

                                return unified;
                            }}
                            ref={loader => {
                                repoLoader.current = loader;
                            }}>
                            {(allRepos: UnifiedRepo[]) => {
                                const filterResults = getRepoFilterResults(allRepos, filterPref);
                                const filteredRepos = filteredName(filterRepos(filterResults), name).sort((a, b) => {
                                    switch (sortKey) {
                                        case 'type':
                                            return compareString(getRepoType(a), getRepoType(b));
                                        case 'name':
                                            return compareString(getRepoName(a), getRepoName(b));
                                        case 'repository':
                                            return compareString(getRepoUrl(a), getRepoUrl(b));
                                        case 'project':
                                            return compareString(getRepoProject(a), getRepoProject(b));
                                        case 'permission':
                                            return compareNumber(Number(isWrite(a)), Number(isWrite(b)));
                                        default:
                                            return 0;
                                    }
                                });

                                return (
                                    <>
                                        {ReactDOM.createPortal(
                                            <DataLoader load={() => services.viewPreferences.getPreferences()}>
                                                {allpref => <ReposFilter repos={filterResults} pref={filterPref} onChange={onFilterChange} collapsed={allpref.hideSidebar} />}
                                            </DataLoader>,
                                            sidebarTarget?.current
                                        )}
                                        {filteredRepos.length > 0 ? (
                                            <Paginate page={page} data={filteredRepos} onPageChange={setPage} preferencesKey='repos-list'>
                                                {reposToDisplay => (
                                                    <div className='argo-table-list argo-table-list--clickable'>
                                                        <div className='argo-table-list__head'>
                                                            <div className='row'>
                                                                <IconColumn />
                                                                <div className='columns small-1 sortable' onClick={() => requestSort('type')}>
                                                                    TYPE
                                                                    {sortIcon('type')}
                                                                </div>
                                                                <div className='columns small-2 sortable' onClick={() => requestSort('name')}>
                                                                    NAME
                                                                    {sortIcon('name')}
                                                                </div>
                                                                <div className='columns small-4 sortable' onClick={() => requestSort('repository')}>
                                                                    REPOSITORY
                                                                    {sortIcon('repository')}
                                                                </div>
                                                                <div className='columns small-1 sortable' onClick={() => requestSort('project')}>
                                                                    PROJECT
                                                                    {sortIcon('project')}
                                                                </div>
                                                                <div className='columns small-1 sortable' onClick={() => requestSort('permission')}>
                                                                    PERMISSION
                                                                    {sortIcon('permission')}
                                                                </div>
                                                                <div className='columns small-2'>CONNECTION STATUS</div>
                                                            </div>
                                                        </div>
                                                        {reposToDisplay.map(repo => {
                                                            const repoUrl = getRepoUrl(repo);
                                                            const repoKey = `${isTemplate(repo) ? 'template' : 'repo'}-${isWrite(repo) ? 'write' : 'read'}-${repoUrl}`;
                                                            const connectionState = getConnectionState(repo);
                                                            return (
                                                                <div className='argo-table-list__row' key={repoKey} onClick={() => displayEditSliding(repo)}>
                                                                    <div className='row'>
                                                                        <IconColumn icon={'argo-icon-' + getRepoType(repo)} />
                                                                        <div className='columns small-1'>
                                                                            <span>{getRepoType(repo)}</span>
                                                                        </div>
                                                                        <div className='columns small-2'>
                                                                            <Tooltip content={getRepoName(repo)}>
                                                                                <span>{getRepoName(repo)}</span>
                                                                            </Tooltip>
                                                                        </div>
                                                                        <div className='columns small-4'>
                                                                            <Tooltip content={repoUrl}>
                                                                                <span>
                                                                                    <Repo url={repoUrl} />
                                                                                </span>
                                                                            </Tooltip>
                                                                        </div>
                                                                        <div className='columns small-1'>
                                                                            <Tooltip content={getRepoProject(repo)}>
                                                                                <span>{getRepoProject(repo)}</span>
                                                                            </Tooltip>
                                                                        </div>
                                                                        <div className='columns small-1'>
                                                                            <span>{isWrite(repo) ? 'Write' : 'Read'}</span>
                                                                        </div>
                                                                        <div className='columns small-2'>
                                                                            {connectionState ? (
                                                                                <>
                                                                                    <ConnectionStateIcon state={connectionState} /> {connectionState.status}
                                                                                </>
                                                                            ) : (
                                                                                <span>-</span>
                                                                            )}
                                                                            {!isTemplate(repo) && (
                                                                                <ActionMenu
                                                                                    items={[
                                                                                        {
                                                                                            title: 'Create application',
                                                                                            action: () =>
                                                                                                ctx.navigation.goto('/applications', {
                                                                                                    new: JSON.stringify({spec: {source: {repoURL: repoUrl}}})
                                                                                                })
                                                                                        },
                                                                                        {
                                                                                            title: 'Disconnect',
                                                                                            action: () => disconnectRepo(repoUrl, getRepoProject(repo), isWrite(repo))
                                                                                        }
                                                                                    ]}
                                                                                />
                                                                            )}
                                                                            {isTemplate(repo) && (
                                                                                <ActionMenu
                                                                                    items={[
                                                                                        {
                                                                                            title: 'Remove',
                                                                                            action: () => removeRepoCreds(repoUrl, isWrite(repo))
                                                                                        }
                                                                                    ]}
                                                                                />
                                                                            )}
                                                                        </div>
                                                                    </div>
                                                                </div>
                                                            );
                                                        })}
                                                    </div>
                                                )}
                                            </Paginate>
                                        ) : allRepos.length === 0 ? (
                                            <EmptyState icon='argo-icon-git'>
                                                <h4>No repositories connected</h4>
                                                <h5>Connect your repo to deploy apps.</h5>
                                            </EmptyState>
                                        ) : (
                                            <EmptyState icon='fa fa-search'>
                                                <h4>No matching repositories found</h4>
                                                <h5>
                                                    Change filter criteria or&nbsp;
                                                    <a
                                                        onClick={() => {
                                                            const newPref = {...filterPref};
                                                            ReposListPreferencesHelper.clearFilters(newPref);
                                                            onFilterChange(newPref);
                                                        }}>
                                                        clear filters
                                                    </a>
                                                </h5>
                                            </EmptyState>
                                        )}
                                    </>
                                );
                            }}
                        </DataLoader>
                    )}
                </div>
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
                header={
                    <SlidingPanelHeader
                        showConnectRepo={showConnectRepo()}
                        displayEditPanel={displayEditPanel}
                        connecting={connecting}
                        onConnect={() => {
                            credsTemplate.current = false;
                            formApi.current.submitForm(null);
                        }}
                        onSaveCredsTemplate={() => {
                            credsTemplate.current = true;
                            formApi.current.submitForm(null);
                        }}
                        setConnectRepo={setConnectRepo}
                        setDisplayEditPanel={setDisplayEditPanel}
                    />
                }>
                {showConnectRepo() && <ConnectRepoFormButton method={method} onSelection={setMethod} formApi={formApi} />}
                {displayEditPanel && currentRepo && (
                    <RepoDetails item={currentRepo} save={(params: NewHTTPSRepoParams) => updateHTTPSRepo(params)} readonly={!isRepoUpdatable(currentRepo)} />
                )}
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
                                                <p>SAVE AS WRITE CREDENTIAL (BETA)</p>
                                                <p>
                                                    The Source Hydrator is a Beta feature which enables Applications to push hydrated manifests to git before syncing. To use Source
                                                    Hydrator for a repository, you must save two credentials: a read credential for pulling manifests and a write credential for
                                                    pushing hydrated manifests. If you add a write credential for a repository, then{' '}
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
                                                <div className='argo-form-row'>
                                                    <FormField formApi={formApi} label='Depth (optional)' field='depth' component={NumberField} />
                                                    <HelpIcon title='Depth for shallow clones. Leave empty or 0 for a full clone.' />
                                                </div>
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
                                                {formApi.getFormState().values.type === 'git' && (
                                                    <div className='argo-form-row'>
                                                        <FormField formApi={formApi} label='Depth (optional)' field='depth' component={NumberField} />
                                                        <HelpIcon title='Depth for shallow clones. Leave empty or 0 for a full clone.' />
                                                    </div>
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
                                                <div className='argo-form-row'>
                                                    <FormField formApi={formApi} label='Depth (optional)' field='depth' component={NumberField} />
                                                    <HelpIcon title='Depth for shallow clones. Leave empty or 0 for a full clone.' />
                                                </div>
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
                                                <div className='argo-form-row'>
                                                    <FormField formApi={formApi} label='Depth (optional)' field='depth' component={NumberField} />
                                                    <HelpIcon title='Depth for shallow clones. Leave empty or 0 for a full clone.' />
                                                </div>
                                            </div>
                                        )}
                                        {method === ConnectionMethod.AZURESERVICEPRINCIPAL && (
                                            <div className='white-box'>
                                                <p>CONNECT REPO USING AZURE SERVICE PRINCIPAL</p>
                                                <div className='argo-form-row'>
                                                    <FormField
                                                        formApi={formApi}
                                                        label='Type'
                                                        field='azureType'
                                                        component={FormSelect}
                                                        componentProps={{options: ['Azure Public Cloud', 'Azure Other Cloud']}}
                                                    />
                                                </div>
                                                {formApi.getFormState().values.azureType === 'Azure Other Cloud' && (
                                                    <div className='argo-form-row'>
                                                        <FormField
                                                            formApi={formApi}
                                                            label='Azure Active Directory Endpoint (e.g. https://login.microsoftonline.de)'
                                                            field='azureActiveDirectoryEndpoint'
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
                                                    <FormField formApi={formApi} label='AzureTenant ID' field='azureServicePrincipalTenantId' component={Text} />
                                                </div>
                                                <div className='argo-form-row'>
                                                    <FormField formApi={formApi} label='Azure Client ID' field='azureServicePrincipalClientId' component={Text} />
                                                </div>
                                                <div className='argo-form-row'>
                                                    <FormField formApi={formApi} label='Azure Client Secret' field='azureServicePrincipalClientSecret' component={Text} />
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
