import * as React from 'react';
import {Sidebar} from '../../../sidebar/sidebar';
import {ViewPreferences, AppSetViewPreferences} from '../../services';

require('./layout.scss');

export interface AbstractLayoutProps {
    navItems: Array<{path: string; iconClassName: string; title: string}>;
    onVersionClick?: () => void;
    children?: React.ReactNode;
    pref: any;
    isExtension?: boolean;
}

export interface LayoutProps extends AbstractLayoutProps {
    pref: ViewPreferences;
}

export interface AppSetLayoutProps extends AbstractLayoutProps {
    pref: AppSetViewPreferences;
}

const getBGColor = (theme: string): string => (theme === 'light' ? '#dee6eb' : '#100f0f');

export const Layout = (props: AbstractLayoutProps) => (
    <div className={props.pref.theme ? 'theme-' + props.pref.theme : 'theme-light'}>
        <div className={`cd-layout ${props.isExtension ? 'cd-layout--extension' : ''}`}>
            <Sidebar onVersionClick={props.onVersionClick} navItems={props.navItems} pref={props.pref} />
            {props.pref.theme ? (document.body.style.background = getBGColor(props.pref.theme)) : null}
            <div className={`cd-layout__content ${props.pref.hideSidebar ? 'cd-layout__content--sb-collapsed' : 'cd-layout__content--sb-expanded'} custom-styles`}>
                {props.children}
            </div>
        </div>
    </div>
);
