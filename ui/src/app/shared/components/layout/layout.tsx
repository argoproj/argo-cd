import * as React from 'react';
import {Sidebar} from '../../../sidebar/sidebar';
import {ViewPreferences} from '../../services';

require('./layout.scss');

export interface LayoutProps {
    navItems: Array<{path: string; iconClassName: string; title: string}>;
    onVersionClick?: () => void;
    children?: React.ReactNode;
    pref: ViewPreferences;
    isExtension?: boolean;
}

export const Layout = (props: LayoutProps) => (
    <div className={props.pref.theme ? 'theme-' + props.pref.theme : 'theme-light'}>
        <div className={`cd-layout ${props.isExtension ? 'cd-layout--extension' : ''}`}>
            <Sidebar onVersionClick={props.onVersionClick} navItems={props.navItems} pref={props.pref} />
            <div className={`cd-layout__content ${props.pref.hideSidebar ? 'cd-layout__content--sb-collapsed' : 'cd-layout__content--sb-expanded'}`}>{props.children}</div>
        </div>
    </div>
);
