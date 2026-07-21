import {useCallback, useSyncExternalStore} from 'react';
import type {CSSProperties} from 'react';
import {AuthSettings, Cluster, UserInfo} from './models';

export function hashCode(str: string) {
    let hash = 0;
    for (let i = 0; i < str.length; i++) {
        // tslint:disable-next-line:no-bitwise
        hash = ~~((hash << 5) - hash + str.charCodeAt(i));
    }
    return hash;
}

// concatMaps merges two maps. Later args take precedence where there's a key conflict.
export function concatMaps(...maps: (Map<string, string> | null)[]): Map<string, string> {
    const newMap = new Map<string, string>();
    for (const map of maps) {
        if (map) {
            for (const entry of Object.entries(map)) {
                newMap.set(entry[0], entry[1]);
            }
        }
    }
    return newMap;
}

export function isValidURL(url: string): boolean {
    try {
        const parsedUrl = new URL(url);
        return parsedUrl.protocol !== 'javascript:' && parsedUrl.protocol !== 'data:' && parsedUrl.protocol !== 'vbscript:';
    } catch {
        try {
            // Try parsing as a relative URL.
            const parsedUrl = new URL(url, window.location.origin);
            return parsedUrl.protocol !== 'javascript:' && parsedUrl.protocol !== 'data:' && parsedUrl.protocol !== 'vbscript:';
        } catch {
            return false;
        }
    }
}

// managed-by-url is expected to mostly if not always point to another Argo CD instance URL,
// so we only consider http/https valid for click-through behavior.
export function isValidManagedByURL(url: string): boolean {
    try {
        const parsedUrl = new URL(url);
        return parsedUrl.protocol === 'http:' || parsedUrl.protocol === 'https:';
    } catch {
        return false;
    }
}

export const MANAGED_BY_URL_INVALID_TEXT = 'managed-by-url: invalid url provided';
export const MANAGED_BY_URL_INVALID_TOOLTIP = 'managed-by-url must be a valid http(s) URL for the managing Argo CD instance. The external link is disabled until this is fixed.';

export const MANAGED_BY_URL_INVALID_COLOR = '#f4c030';

export const managedByURLInvalidLabelStyle: CSSProperties = {
    color: MANAGED_BY_URL_INVALID_COLOR,
    marginLeft: '0.5em',
    fontSize: '13px',
    fontWeight: 500,
    lineHeight: 1.35,
    whiteSpace: 'nowrap'
};

export const managedByURLInvalidLabelStyleCompact: CSSProperties = {
    ...managedByURLInvalidLabelStyle,
    marginLeft: '4px',
    fontSize: '12px',
    fontWeight: 600
};

export const colorSchemes = {
    light: '(prefers-color-scheme: light)',
    dark: '(prefers-color-scheme: dark)'
};

/**
 * quick method to check system theme
 * @param theme auto, light, dark
 * @returns dark or light
 */
export function getTheme(theme: string) {
    if (theme !== 'auto') {
        return theme;
    }

    const dark = window.matchMedia(colorSchemes.dark);

    return dark.matches ? 'dark' : 'light';
}

/**
 * create a listener for system theme
 * @param cb callback for theme change
 * @returns destroy listener
 */
const subscribeSystemTheme = (cb: (theme: string) => void) => {
    const dark = window.matchMedia(colorSchemes.dark);
    const light = window.matchMedia(colorSchemes.light);

    const listener = () => {
        cb(dark.matches ? 'dark' : 'light');
    };

    dark.addEventListener('change', listener);
    light.addEventListener('change', listener);

    return () => {
        dark.removeEventListener('change', listener);
        light.removeEventListener('change', listener);
    };
};

export const createSystemThemeListener = (cb: (theme: string) => void) => subscribeSystemTheme(cb);

export const useTheme = (props: {theme: string}) => {
    // subscribe to system theme changes, but only register a listener when theme is auto
    const subscribe = useCallback(
        (onStoreChange: () => void) => {
            if (props.theme !== 'auto') {
                return () => undefined;
            }
            return subscribeSystemTheme(() => onStoreChange());
        },
        [props.theme]
    );

    // derive the resolved theme during render so manual theme changes are reflected immediately
    const getSnapshot = useCallback(() => getTheme(props.theme), [props.theme]);

    const theme = useSyncExternalStore(subscribe, getSnapshot);

    return [theme];
};

export const formatClusterQueryParam = (cluster: Cluster) => {
    if (cluster.name === cluster.server) {
        return cluster.name;
    }
    return `${cluster.name} (${cluster.server})`;
};

export const isInvalidRegex = (pattern: string): boolean => {
    if (!pattern) {
        return false;
    }
    try {
        new RegExp(pattern);
        return false;
    } catch {
        return true;
    }
};

/**
 * Checks if SSO is configured for authentication usage.
 * @param userInfo - User information from the session
 * @param authSettings - Authentication settings from the server
 * @returns true if SSO should be used, otherwise false
 */
export function isSSOConfigured(userInfo: UserInfo | null | undefined, authSettings: AuthSettings): boolean {
    const isExternalIssuer = userInfo?.iss && userInfo.iss !== 'argocd';
    const hasDexConnectors = (authSettings.dexConfig?.connectors?.length ?? 0) > 0;
    const hasOidcConfig = !!authSettings.oidcConfig;
    return isExternalIssuer && (hasDexConnectors || hasOidcConfig);
}
