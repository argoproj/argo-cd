import {Tooltip} from 'argo-ui';
import {useData} from 'argo-ui/v2';
import * as React from 'react';
import {Context} from '../shared/context';
import {services, ViewPreferences} from '../shared/services';

require('./sidebar.scss');

interface SidebarProps {
    onVersionClick: () => void;
    navItems: {path: string; iconClassName: string; title: string; tooltip?: string}[];
    pref: ViewPreferences;
}

export const SIDEBAR_TOOLS_ID = 'sidebar-tools';

export const useSidebarTarget = () => {
    const sidebarTarget = React.useRef(document.createElement('div'));

    React.useEffect(() => {
        const sidebar = document.getElementById(SIDEBAR_TOOLS_ID);
        sidebar.appendChild(sidebarTarget?.current);
        return () => {
            sidebarTarget.current?.remove();
        };
    }, []);

    return sidebarTarget;
};

export const Sidebar = (props: SidebarProps) => {
    const context = React.useContext(Context);
    const [version, loading, error] = useData(() => services.version.version());
    const locationPath = context.history.location.pathname;

    const tooltipProps = {
        placement: 'right',
        popperOptions: {
            modifiers: {
                preventOverflow: {
                    boundariesElement: 'window'
                }
            }
        }
    };

    return (
        <div className={`sidebar ${props.pref.hideSidebar ? 'sidebar--collapsed' : ''}`}>
            <div className='sidebar__logo'>
                <img src='assets/images/logo.png' alt='Argo' /> {!props.pref.hideSidebar && 'Argo CD'}
            </div>
            <div className='sidebar__version' onClick={props.onVersionClick}>
                {loading ? 'Loading...' : error?.state ? 'Unknown' : version?.Version || 'Unknown'}
            </div>
            {(props.navItems || []).map(item => (
                <Tooltip key={item.path} content={item?.tooltip || item.title} {...tooltipProps}>
                    <div
                        key={item.title}
                        className={`sidebar__nav-item ${locationPath === item.path || locationPath.startsWith(`${item.path}/`) ? 'sidebar__nav-item--active' : ''}`}
                        onClick={() => context.history.push(item.path)}>
                        <React.Fragment>
                            <div>
                                <i className={item?.iconClassName || ''} />
                                {!props.pref.hideSidebar && item.title}
                            </div>
                        </React.Fragment>
                    </div>
                </Tooltip>
            ))}
            <div onClick={() => services.viewPreferences.updatePreferences({...props.pref, hideSidebar: !props.pref.hideSidebar})} className='sidebar__collapse-button'>
                <i className={`fas fa-arrow-${props.pref.hideSidebar ? 'right' : 'left'}`} />
            </div>
            {props.pref.hideSidebar && (
                <div onClick={() => services.viewPreferences.updatePreferences({...props.pref, hideSidebar: !props.pref.hideSidebar})} className='sidebar__collapse-button'>
                    <Tooltip content='Show Filters' {...tooltipProps}>
                        <div className='sidebar__nav-item'>
                            <i className={`fas fa-filter`} style={{fontSize: '14px', margin: '0 auto'}} />
                        </div>
                    </Tooltip>
                </div>
            )}
            <div id={SIDEBAR_TOOLS_ID} />
        </div>
    );
};
