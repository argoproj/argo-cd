import {useData} from 'argo-ui/v2';
import * as React from 'react';
import {Context} from '../shared/context';
import {services} from '../shared/services';

require('./sidebar.scss');

interface SidebarProps {
    onVersionClick: () => void;
    navItems: {path: string; iconClassName: string; title: string}[];
}

export const Sidebar = (props: SidebarProps) => {
    const context = React.useContext(Context);
    const [version, loading, error] = useData(() => services.version.version());
    const locationPath = context.history.location.pathname;

    const [collapsed, setCollapsed] = React.useState(false);

    return (
        <div className={`sidebar ${collapsed ? 'sidebar--collapsed' : ''}`}>
            <div className='sidebar__logo'>
                <img src='assets/images/logo.png' alt='Argo' /> {!collapsed && 'Argo CD'}
            </div>
            <div className='sidebar__version' onClick={props.onVersionClick}>
                {loading ? 'Loading...' : error?.state ? 'Unknown' : version?.Version || 'Unknown'}
            </div>
            {(props.navItems || []).map(item => (
                <div
                    key={item.title}
                    className={`sidebar__nav-item ${locationPath === item.path || locationPath.startsWith(`${item.path}/`) ? 'sidebar__nav-item--active' : ''}`}
                    onClick={() => context.history.push(item.path)}>
                    <i className={item.iconClassName} /> {!collapsed && item.title}
                </div>
            ))}
            <div onClick={() => setCollapsed(!collapsed)} className='sidebar__collapse-button'>
                <i className={`fas fa-arrow-${collapsed ? 'right' : 'left'}`} />
            </div>
        </div>
    );
};
