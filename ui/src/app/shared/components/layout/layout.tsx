import * as React from 'react';
import {Sidebar} from '../../../sidebar/sidebar';
import {ViewPreferences} from '../../services';
import {useTheme} from '../../utils';

require('./layout.scss');

export interface LayoutProps {
    navItems: Array<{path: string; iconClassName: string; title: string}>;
    onVersionClick?: () => void;
    children?: React.ReactNode;
    pref: ViewPreferences;
}

const getBGColor = (theme: string): string => (theme === 'light' ? '#dee6eb' : '#100f0f');

export const ThemeWrapper = (props: {children: React.ReactNode; theme: string}) => {
    const [theme] = useTheme({theme: 'auto'});
    return <div className={'theme-' + theme}>{props.children}</div>;
};

export const Layout = (props: LayoutProps) => {
    const [theme] = useTheme({theme: props.pref.theme});
    React.useEffect(() => {
        if (theme) {
            document.body.style.background = getBGColor(theme);
        }
    }, [theme]);

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
