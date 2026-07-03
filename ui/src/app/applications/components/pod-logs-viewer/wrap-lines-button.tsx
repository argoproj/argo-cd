import {services, ViewPreferences} from '../../../shared/services';
import * as React from 'react';
import {ToggleButton} from '../../../shared/components/toggle-button';

// WrapLinesButton is a component that wraps log lines.
export const WrapLinesButton = ({prefs}: {prefs: ViewPreferences}) => (
    <ToggleButton
        title='Wrap Lines'
        onToggle={() => {
            const wrap = prefs.appDetails.wrapLines;
            services.viewPreferences.updatePreferences({...prefs, appDetails: {...prefs.appDetails, wrapLines: !wrap}});
        }}
        toggled={prefs.appDetails.wrapLines}
        icon='share'
        rotate={true}
    />
);
