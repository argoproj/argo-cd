import * as React from 'react';
import {Sidebar} from '../../../sidebar/sidebar';
import {ViewPreferences} from '../../services';
import {useTheme} from '../../utils';

require('./layout.scss');
require('../../dark-mode-overrides.scss');

export interface LayoutProps {
    navItems: Array<{path: string; iconClassName: string; title: string}>;
    onVersionClick?: () => void;
    children?: React.ReactNode;
    pref: ViewPreferences;
}

export const useBodyTheme = (themePref: string) => {
    const [theme] = useTheme({theme: themePref});
    React.useEffect(() => {
        if (theme) {
            document.body.style.background = theme === 'light' ? '#dee6eb' : '#141a20';
            document.body.classList.toggle('dark-theme', theme === 'dark');
        }
    }, [theme]);
    return theme;
};

export const ThemeWrapper = (props: {children: React.ReactNode; theme: string}) => {
    const [systemTheme] = useTheme({
        theme: props.theme
    });
    return <div className={'theme-' + systemTheme}>{props.children}</div>;
};

export const Layout = (props: LayoutProps) => {
    const theme = useBodyTheme(props.pref.theme);

    return (
        <div className={`theme-${theme}`}>
            <div className={'cd-layout'}>
                <Sidebar onVersionClick={props.onVersionClick} navItems={props.navItems} pref={props.pref} />
                <div className={`cd-layout__content ${props.pref.hideSidebar ? 'cd-layout__content--sb-collapsed' : 'cd-layout__content--sb-expanded'} custom-styles`}>
                    {props.children}
                </div>
            </div>
        </div>
    );
};
