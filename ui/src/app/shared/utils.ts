import React from 'react';

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
    } catch (TypeError) {
        try {
            // Try parsing as a relative URL.
            const parsedUrl = new URL(url, window.location.origin);
            return parsedUrl.protocol !== 'javascript:' && parsedUrl.protocol !== 'data:' && parsedUrl.protocol !== 'vbscript:';
        } catch (TypeError) {
            return false;
        }
    }
}

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
export const useSystemTheme = (cb: (theme: string) => void) => {
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

export const useTheme = (props: {theme: string}) => {
    const [theme, setTheme] = React.useState(getTheme(props.theme));

    React.useEffect(() => {
        let destroyListener: (() => void) | undefined;

        // change theme by system, only register listener when theme is auto
        if (props.theme === 'auto') {
            destroyListener = useSystemTheme(systemTheme => {
                setTheme(systemTheme);
            });
        }

        // change theme manually
        if (props.theme !== theme) {
            setTheme(getTheme(props.theme));
        }

        return () => {
            destroyListener?.();
        };
    }, [props.theme]);

    return [theme];
};
