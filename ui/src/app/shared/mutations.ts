import {useMutation, useQueryClient} from '@tanstack/react-query';

import {Cluster, GnuPGPublicKey, Project} from './models';
import {clusterQueries, gpgkeyQueries, projectQueries} from './queries';
import {CreateJWTTokenParams, DeleteJWTTokenParams, ProjectRoleParams, ProjectSyncWindowsParams, services} from './services';

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
