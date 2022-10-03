import * as React from 'react';
import {Sidebar} from '../../../sidebar/sidebar';

require('./layout.scss');

export interface LayoutProps {
    navItems: Array<{path: string; iconClassName: string; title: string}>;
    onVersionClick?: () => void;
    theme?: string;
    children?: React.ReactNode;
}

export const Layout = (props: LayoutProps) => (
    <div className={props.theme ? 'theme-' + props.theme : 'theme-light'}>
        <div className='cd-layout'>
            <Sidebar onVersionClick={props.onVersionClick} navItems={props.navItems} />
            {props.children}
        </div>
    </div>
);
