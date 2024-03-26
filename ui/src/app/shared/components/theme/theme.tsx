import * as React from 'react';
import {ViewPreferences} from '../../services';
import {ReactNode, useState} from 'react';

export const useTheme = (pref: ViewPreferences) => {
    const [theme, setTheme] = useState(pref.selectedTheme);

    React.useEffect(() => {
        if (pref.useSystemTheme) {
            const darkThemeMatcher = window.matchMedia('(prefers-color-scheme: dark)');
            if (darkThemeMatcher.matches) {
                setTheme('dark');
            } else {
                setTheme('light');
            }
        } else {
            setTheme(pref.selectedTheme);
        }
    }, [pref.useSystemTheme, pref.selectedTheme, setTheme]);

    return theme;
};

export const Theme = (props: {pref: ViewPreferences; children?: ReactNode}) => {
    const theme = useTheme(props.pref);

    return <div className={'theme-' + theme}>{props.children}</div>;
};
