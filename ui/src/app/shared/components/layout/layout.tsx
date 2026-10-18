import * as React from 'react';
import {Sidebar, SidebarToolsContext} from '../../../sidebar/sidebar';
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
    const [systemTheme] = useTheme({
        theme: props.theme
    });
    return <div className={'theme-' + systemTheme}>{props.children}</div>;
};

export const Layout = (props: LayoutProps) => {
    const [theme] = useTheme({theme: props.pref.theme});
    // The filter slot lives in the sidebar but is consumed by the page, so `Layout` owns it and hands it
    // down through context instead of letting pages create their own container outside the React tree.
    const [toolsEl, setToolsEl] = React.useState<HTMLDivElement | null>(null);
    React.useEffect(() => {
        if (theme) {
            document.body.style.background = getBGColor(theme);
        }
    }, [theme]);

    return (
        <div className={`theme-${theme}`}>
            <div className={'cd-layout'}>
                <Sidebar onVersionClick={props.onVersionClick} navItems={props.navItems} pref={props.pref} onToolsElement={setToolsEl} />
                <div className={`cd-layout__content ${props.pref.hideSidebar ? 'cd-layout__content--sb-collapsed' : 'cd-layout__content--sb-expanded'} custom-styles`}>
                    <SidebarToolsContext.Provider value={toolsEl}>{props.children}</SidebarToolsContext.Provider>
                </div>
            </div>
        </div>
    );
};
