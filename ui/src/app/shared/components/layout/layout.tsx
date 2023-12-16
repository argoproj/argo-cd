import * as React from 'react';
import {Sidebar} from '../../../sidebar/sidebar';
import {ViewPreferences} from '../../services';
import {Theme, useTheme} from '../theme/theme';

require('./layout.scss');

export interface LayoutProps {
    navItems: Array<{path: string; iconClassName: string; title: string}>;
    onVersionClick?: () => void;
    children?: React.ReactNode;
    pref: ViewPreferences;
    isExtension?: boolean;
}

const getBGColor = (theme: string): string => (theme === 'light' ? '#dee6eb' : '#100f0f');

export const Layout = (props: LayoutProps) => {
    const theme = useTheme(props.pref);

    React.useEffect(() => {
        if (theme) {
            document.body.style.background = getBGColor(theme);
        }
    }, [theme]);

    return (
        <Theme pref={props.pref}>
            <div className={`cd-layout ${props.isExtension ? 'cd-layout--extension' : ''}`}>
                <Sidebar onVersionClick={props.onVersionClick} navItems={props.navItems} pref={props.pref} />
                <div className={`cd-layout__content ${props.pref.hideSidebar ? 'cd-layout__content--sb-collapsed' : 'cd-layout__content--sb-expanded'} custom-styles`}>
                    {props.children}
                </div>
            </div>
        </Theme>
    );
};
