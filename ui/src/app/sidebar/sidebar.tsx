import {DataLoader} from 'argo-ui';
 import {useData} from 'argo-ui/v2';
 import * as React from 'react';
 import {Context} from '../shared/context';
 import {services} from '../shared/services';

 require('./sidebar.scss');

 interface SidebarProps {
     onVersionClick: () => void;
     navItems: {path: string; iconClassName: string; title: string}[];
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

     return (
         <DataLoader load={() => services.viewPreferences.getPreferences()}>
             {pref => (
                 <div className={`sidebar ${pref.hideSidebar ? 'sidebar--collapsed' : ''}`}>
                     <div className='sidebar__logo'>
                         <img src='assets/images/logo.png' alt='Argo' /> {!pref.hideSidebar && 'Argo CD'}
                     </div>
                     <div className='sidebar__version' onClick={props.onVersionClick}>
                         {loading ? 'Loading...' : error?.state ? 'Unknown' : version?.Version || 'Unknown'}
                     </div>
                     {(props.navItems || []).map(item => (
                         <div
                             key={item.title}
                             className={`sidebar__nav-item ${locationPath === item.path || locationPath.startsWith(`${item.path}/`) ? 'sidebar__nav-item--active' : ''}`}
                             onClick={() => context.history.push(item.path)}>
                             <i className={item.iconClassName} /> {!pref.hideSidebar && item.title}
                         </div>
                     ))}
                     <div onClick={() => services.viewPreferences.updatePreferences({...pref, hideSidebar: !pref.hideSidebar})} className='sidebar__collapse-button'>
                         <i className={`fas fa-arrow-${pref.hideSidebar ? 'right' : 'left'}`} />
                     </div>
                     <div id={SIDEBAR_TOOLS_ID} />
                 </div>
             )}
         </DataLoader>
     );
 };