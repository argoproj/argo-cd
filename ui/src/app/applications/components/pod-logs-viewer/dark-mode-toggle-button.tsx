import {services, ViewPreferences} from '../../../shared/services';
import * as React from 'react';
import {ToggleButton} from '../../../shared/components/toggle-button';

// DarkModeToggleButton is a component that renders a toggle button that toggles dark mode.
// Added a logviewer theme profile

interface LogViewerTheme {
    light: boolean;
    dark: boolean;
}

export const DarkModeToggleButton = ({prefs}: {prefs: ViewPreferences}) => {
    React.useEffect(() => {
        const profiles = prefs.appDetails.LogViewerTheme as LogViewerTheme;
        const currentTheme = prefs.theme;

        if (!profiles) {
            services.viewPreferences.updatePreferences({
                ...prefs,
                appDetails: {
                    ...prefs.appDetails,
                    LogViewerTheme: {
                        light: false,
                        dark: true
                    },
                    darkMode: currentTheme === 'dark'
                }
            });
            return;
        }

        const profilePreference = currentTheme === 'dark' ? profiles.dark : profiles.light;
        if (prefs.appDetails.darkMode !== profilePreference) {
            services.viewPreferences.updatePreferences({
                ...prefs,
                appDetails: {
                    ...prefs.appDetails,
                    darkMode: profilePreference
                }
            });
        }
    }, [prefs.theme]);

    const handleToggle = () => {
        const newDarkMode = !prefs.appDetails.darkMode;
        const currentTheme = prefs.theme;
        const profiles = (prefs.appDetails.LogViewerTheme as LogViewerTheme) || {
            light: false,
            dark: true
        };

        services.viewPreferences.updatePreferences({
            ...prefs,
            appDetails: {
                ...prefs.appDetails,
                darkMode: newDarkMode,
                LogViewerTheme: {
                    ...profiles,
                    [currentTheme]: newDarkMode
                }
            }
        });
    };

    return <ToggleButton title='Dark Mode' onToggle={handleToggle} toggled={prefs.appDetails.darkMode} icon='moon' />;
};
