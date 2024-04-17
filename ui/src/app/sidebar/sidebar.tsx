import {Tooltip} from 'argo-ui';
import {Boundary, Placement} from 'popper.js';
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
        placement: 'right' as Placement,
        popperOptions: {
            modifiers: {
                preventOverflow: {
                    boundariesElement: 'window' as Boundary
                }
            }
        }
    };

    return (
        <div className={`sidebar ${props.pref.hideSidebar ? 'sidebar--collapsed' : ''}`}>
            <div className='sidebar__container'>
                <div className='sidebar__logo'>
                    <div onClick={() => services.viewPreferences.updatePreferences({...props.pref, hideSidebar: !props.pref.hideSidebar})} className='sidebar__collapse-button'>
                        <i className={`fas fa-arrow-${props.pref.hideSidebar ? 'right' : 'left'}`} />
                    </div>
                    {!props.pref.hideSidebar && (
                        <div className='sidebar__logo-container'>
                            <img src='assets/images/argologo.svg' alt='Argo' className='sidebar__logo__text-logo' />
                            <div className='sidebar__version' onClick={props.onVersionClick}>
                                {loading ? 'Loading...' : error?.state ? 'Unknown' : version?.Version || 'Unknown'}
                            </div>
                        </div>
                    )}
                    <img src='assets/images/logo.png' alt='Argo' className='sidebar__logo__character' />{' '}
                </div>

                {(props.navItems || []).map(item => (
                    <Tooltip key={item.path} content={<div className='sidebar__tooltip'>{item?.tooltip || item.title}</div>} {...tooltipProps}>
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

                {props.pref.hideSidebar && (
                    <Tooltip content='Show Filters' {...tooltipProps}>
                        <div
                            onClick={() => services.viewPreferences.updatePreferences({...props.pref, hideSidebar: !props.pref.hideSidebar})}
                            className='sidebar__nav-item sidebar__filter-button'>
                            <div>
                                <i className={`fas fa-filter`} />
                            </div>
                        </div>
                    </Tooltip>
                )}
            </div>
            <div id={SIDEBAR_TOOLS_ID} />
        </div>
    );
};
