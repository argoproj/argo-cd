import * as React from 'react';
import {AppDetailsPreferences} from '../../../shared/services';
import {services} from '../../../shared/services';

export const StatusPanelToggle = ({pref}: {pref: AppDetailsPreferences}) => (
    <button
        className='application-details__status-panel-toggle'
        title={pref.hideStatusPanel ? 'Expand status panel' : 'Collapse status panel'}
        onClick={() => services.viewPreferences.updatePreferences({appDetails: {...pref, hideStatusPanel: !pref.hideStatusPanel}})}>
        <i className={`fa fa-chevron-${pref.hideStatusPanel ? 'down' : 'up'}`} />
    </button>
);
