import {useMutation, useQueryClient} from '@tanstack/react-query';

import {Cluster, GnuPGPublicKey, Project, RepoCertList} from './models';
import {certQueries, clusterQueries, gpgkeyQueries, projectQueries, repoQueries} from './queries';
import {CreateJWTTokenParams, DeleteJWTTokenParams, ProjectRoleParams, ProjectSyncWindowsParams, services} from './services';
import {AzureServicePrincipalQuery, GitHubAppQuery, GoogleCloudSourceQuery, HTTPSQuery, SSHQuery} from './services/repo-service';
import {AzureServicePrincipalCreds, GitHubAppCreds, GoogleCloudSourceCreds, HTTPSCreds, SSHCreds} from './services/repocreds-service';

// Each mutation owns its cache update. onSuccess returns the invalidation promise, so mutateAsync
// resolves only after active queries have refetched. Error handling stays with the caller.

export function useProjectMutations(name: string) {
    const queryClient = useQueryClient();
    // Prefix also covers the sync window states and links queries.
    const onSuccess = () => queryClient.invalidateQueries({queryKey: projectQueries.byName(name)});
    return {
        update: useMutation({mutationFn: (project: Project) => services.projects.update(project), onSuccess}),
        updateRole: useMutation({mutationFn: (params: ProjectRoleParams) => services.projects.updateRole(params), onSuccess}),
        updateWindow: useMutation({mutationFn: (params: ProjectSyncWindowsParams) => services.projects.updateWindow(params), onSuccess}),
        createJWTToken: useMutation({mutationFn: (params: CreateJWTTokenParams) => services.projects.createJWTToken(params), onSuccess}),
        deleteJWTToken: useMutation({mutationFn: (params: DeleteJWTTokenParams) => services.projects.deleteJWTToken(params), onSuccess}),
        // Invalidating the detail queries here would refetch a 404 before the caller navigates away.
        delete: useMutation({
            mutationFn: () => services.projects.delete(name),
            onSuccess: () => queryClient.invalidateQueries({queryKey: projectQueries.list().queryKey})
        })
    };
}

export function useClusterMutations() {
    const queryClient = useQueryClient();
    const setDetail = (updated: Cluster) => queryClient.setQueryData(clusterQueries.detail(updated.server).queryKey, updated);
    return {
        update: useMutation({mutationFn: ({cluster, paths}: {cluster: Cluster; paths: string[]}) => services.clusters.update(cluster, ...paths), onSuccess: setDetail}),
        invalidateCache: useMutation({mutationFn: (server: string) => services.clusters.invalidateCache(server), onSuccess: setDetail}),
        // Refreshes after a failed delete too, as the list page did before.
        delete: useMutation({
            mutationFn: (server: string) => services.clusters.delete(server),
            onSettled: () => queryClient.invalidateQueries({queryKey: clusterQueries.list().queryKey})
        })
    };
}

export function useGpgKeyMutations() {
    const queryClient = useQueryClient();
    const onSuccess = () => queryClient.invalidateQueries({queryKey: gpgkeyQueries.list().queryKey});
    return {
        create: useMutation({mutationFn: (key: GnuPGPublicKey) => services.gpgkeys.create(key), onSuccess}),
        delete: useMutation({mutationFn: (keyID: string) => services.gpgkeys.delete(keyID), onSuccess})
    };
}

// write picks the write-credential variant of each call.
type Write = {write: boolean};

export function useRepoMutations() {
    const queryClient = useQueryClient();
    // Repositories and credential templates share one list query.
    const onSuccess = () => queryClient.invalidateQueries({queryKey: repoQueries.all});
    const {repos, repocreds} = services;
    return {
        createSSH: useMutation({mutationFn: (q: SSHQuery & Write) => (q.write ? repos.createSSHWrite(q) : repos.createSSH(q)), onSuccess}),
        createHTTPS: useMutation({mutationFn: (q: HTTPSQuery & Write) => (q.write ? repos.createHTTPSWrite(q) : repos.createHTTPS(q)), onSuccess}),
        createGitHubApp: useMutation({mutationFn: (q: GitHubAppQuery & Write) => (q.write ? repos.createGitHubAppWrite(q) : repos.createGitHubApp(q)), onSuccess}),
        createGoogleCloudSource: useMutation({
            mutationFn: (q: GoogleCloudSourceQuery & Write) => (q.write ? repos.createGoogleCloudSourceWrite(q) : repos.createGoogleCloudSource(q)),
            onSuccess
        }),
        createAzureServicePrincipal: useMutation({
            mutationFn: (q: AzureServicePrincipalQuery & Write) => (q.write ? repos.createAzureServicePrincipalWrite(q) : repos.createAzureServicePrincipal(q)),
            onSuccess
        }),
        delete: useMutation({
            mutationFn: ({url, project, write}: {url: string; project: string} & Write) => (write ? repos.deleteWrite(url, project) : repos.delete(url, project)),
            onSuccess
        }),
        createSSHCreds: useMutation({mutationFn: (c: SSHCreds & Write) => (c.write ? repocreds.createSSHWrite(c) : repocreds.createSSH(c)), onSuccess}),
        createHTTPSCreds: useMutation({mutationFn: (c: HTTPSCreds & Write) => (c.write ? repocreds.createHTTPSWrite(c) : repocreds.createHTTPS(c)), onSuccess}),
        createGitHubAppCreds: useMutation({mutationFn: (c: GitHubAppCreds & Write) => (c.write ? repocreds.createGitHubAppWrite(c) : repocreds.createGitHubApp(c)), onSuccess}),
        createGoogleCloudSourceCreds: useMutation({
            mutationFn: (c: GoogleCloudSourceCreds & Write) => (c.write ? repocreds.createGoogleCloudSourceWrite(c) : repocreds.createGoogleCloudSource(c)),
            onSuccess
        }),
        createAzureServicePrincipalCreds: useMutation({
            mutationFn: (c: AzureServicePrincipalCreds & Write) => (c.write ? repocreds.createAzureServicePrincipalWrite(c) : repocreds.createAzureServicePrincipal(c)),
            onSuccess
        }),
        deleteCreds: useMutation({mutationFn: ({url, write}: {url: string} & Write) => (write ? repocreds.deleteWrite(url) : repocreds.delete(url)), onSuccess})
    };
}

export function useCertMutations() {
    const queryClient = useQueryClient();
    const onSuccess = () => queryClient.invalidateQueries({queryKey: certQueries.list().queryKey});
    return {
        create: useMutation({mutationFn: (certificates: RepoCertList) => services.certs.create(certificates), onSuccess}),
        delete: useMutation({
            mutationFn: ({serverName, certType, certSubType}: {serverName: string; certType: string; certSubType: string}) =>
                services.certs.delete(serverName, certType, certSubType),
            onSuccess
        })
    };
}
