import {services, ViewPreferences} from '../../../shared/services';
import * as React from 'react';
import {ToggleButton} from '../../../shared/components/toggle-button';

export const WrapLinesToggleButton = ({prefs}: {prefs: ViewPreferences}) => (
    <ToggleButton
        title='Wrap Lines'
        onToggle={() => {
            const wrap = prefs.appDetails.wrapLines;
            services.viewPreferences.updatePreferences({...prefs, appDetails: {...prefs.appDetails, wrapLines: !wrap}});
        }}
        toggled={prefs.appDetails.wrapLines}
        icon='paragraph'
    />
);
