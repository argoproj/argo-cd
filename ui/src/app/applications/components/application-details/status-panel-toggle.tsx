import * as React from 'react';

import {AppDetailsPreferences} from '../../../shared/services';
import {useStatusPanelCollapsed} from './use-status-panel-collapsed';

export const StatusPanelToggle = ({collapsed, onToggle}: {collapsed: boolean; onToggle: () => void}) => (
    <button className='application-details__status-panel-toggle' title={collapsed ? 'Expand status panel' : 'Collapse status panel'} onClick={onToggle}>
        <i className={`fa fa-chevron-${collapsed ? 'down' : 'up'}`} />
    </button>
);

/** Renders the status panel and its collapse control, auto-collapsing on short windows without writing the persisted preference. */
export const StatusPanelWithToggle = ({pref, children}: {pref: AppDetailsPreferences; children: (collapsed: boolean) => React.ReactNode}) => {
    const {collapsed, onToggle} = useStatusPanelCollapsed(pref);
    return (
        <div className='application-details__status-panel'>
            {children(collapsed)}
            <StatusPanelToggle collapsed={collapsed} onToggle={onToggle} />
        </div>
    );
};
