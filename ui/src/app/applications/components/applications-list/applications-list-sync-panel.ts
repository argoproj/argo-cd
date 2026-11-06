/**
 * URL parameters that open the Sync panel for one application. The namespace goes under its own key
 * because `appNamespace` on this page filters the applications list: reusing it would keep the list
 * restricted to the synced application's namespace on every reload, even after the panel is closed.
 **/
export function openSyncPanelParams(appName: string, appNamespace: string): {[key: string]: string} {
    return {syncApp: appName, syncAppNamespace: appNamespace};
}

export const closeSyncPanelParams: {[key: string]: string} = {syncApp: null, syncAppNamespace: null};

/**
 * Reads the application the Sync panel is open for. Links saved before the namespace moved to
 * `syncAppNamespace` carry it in `appNamespace`; without the fallback the panel would look the
 * application up in the control plane namespace, which may hold a different application of the same name.
 **/
export function syncPanelTarget(params: URLSearchParams): {appName: string; appNamespace: string} {
    return {appName: params.get('syncApp'), appNamespace: params.get('syncAppNamespace') ?? params.get('appNamespace')};
}
