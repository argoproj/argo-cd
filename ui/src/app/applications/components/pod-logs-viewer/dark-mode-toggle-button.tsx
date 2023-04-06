import {services, ViewPreferences} from '../../../shared/services';
import * as React from 'react';
import {ToggleButton} from '../../../shared/components/toggle-button';

// DarkModeToggleButton is a component that renders a toggle button that toggles dark mode.
export const DarkModeToggleButton = ({prefs}: {prefs: ViewPreferences}) => (
    <ToggleButton
        title='Dark Mode'
        onToggle={() => {
            const inverted = prefs.appDetails.darkMode;
            services.viewPreferences.updatePreferences({
                ...prefs,
                appDetails: {...prefs.appDetails, darkMode: !inverted}
            });
        }}
        toggled={prefs.appDetails.darkMode}
        icon='moon'
    />
);
