import classNames from 'classnames';
import * as React from 'react';
import {AppDetailsPreferences} from '../../../shared/services';
import {services} from '../../../shared/services';

export const StatusPanelToggle = ({pref}: {pref: AppDetailsPreferences}) => (
    <button
        className={classNames('application-details__status-panel-toggle', {'application-details__status-panel-toggle--collapsed': pref.hideStatusPanel})}
        title={pref.hideStatusPanel ? 'Expand status panel' : 'Collapse status panel'}
        onClick={() => services.viewPreferences.updatePreferences({appDetails: {...pref, hideStatusPanel: !pref.hideStatusPanel}})}>
        <i className={`fa fa-chevron-${pref.hideStatusPanel ? 'down' : 'up'}`} />
    </button>
);
