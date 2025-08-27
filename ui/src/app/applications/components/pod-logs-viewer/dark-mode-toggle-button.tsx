import {services, ViewPreferences} from '../../../shared/services';
import * as React from 'react';
import {ToggleButton} from '../../../shared/components/toggle-button';

// DarkModeToggleButton is a component that renders a toggle button that toggles dark mode.
// Added a logviewer theme profile

export const DarkModeToggleButton = ({prefs}: {prefs: ViewPreferences}) => {
    React.useEffect(() => {
        // Only run when theme changes or LogViewerTheme is not initialized
        const profile = prefs.appDetails.UseDarkModeInLogViewerForAppTheme;

        if (!profile) {
            // First time initialization
            services.viewPreferences.updatePreferences({
                ...prefs,
                appDetails: {
                    ...prefs.appDetails,
                    UseDarkModeInLogViewerForAppTheme: {
                        light: prefs.appDetails.darkMode,
                        dark: prefs.appDetails.darkMode
                    }
                }
            });
            return;
        }

        // Update darkMode based on saved preference for current theme
        const currentThemeDarkMode = profile[prefs.theme as keyof typeof profile];
        if (prefs.appDetails.darkMode !== currentThemeDarkMode) {
            services.viewPreferences.updatePreferences({
                ...prefs,
                appDetails: {
                    ...prefs.appDetails,
                    darkMode: currentThemeDarkMode
                }
            });
        }
    }, [prefs.theme]);

    const handleToggle = () => {
        const newDarkMode = !prefs.appDetails.darkMode;
        const currentProfile = prefs.appDetails.UseDarkModeInLogViewerForAppTheme || {
            light: prefs.appDetails.darkMode,
            dark: prefs.appDetails.darkMode
        };

        services.viewPreferences.updatePreferences({
            ...prefs,
            appDetails: {
                ...prefs.appDetails,
                darkMode: newDarkMode,
                UseDarkModeInLogViewerForAppTheme: {
                    ...currentProfile,
                    [prefs.theme]: newDarkMode
                }
            }
        });
    };

    return <ToggleButton title='Dark Mode' onToggle={handleToggle} toggled={prefs.appDetails.darkMode} icon='moon' />;
};
