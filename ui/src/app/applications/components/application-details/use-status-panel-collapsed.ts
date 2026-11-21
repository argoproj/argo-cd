import * as React from 'react';

import {useShortWindow} from '../../../shared/hooks/use-short-window';
import {AppDetailsPreferences, services} from '../../../shared/services';

/**
 * Collapses the application status panel on short windows without writing the
 * persisted hideStatusPanel preference, so other (taller) windows are unaffected.
 * An explicit user preference to hide the panel still wins. Expanding or collapsing
 * the panel on a short window is a per-window override until the window is tall again.
 */
export function useStatusPanelCollapsed(pref: AppDetailsPreferences): {collapsed: boolean; onToggle: () => void} {
    const isShortWindow = useShortWindow();
    const [expandedOnShortWindow, setExpandedOnShortWindow] = React.useState(false);

    // Drop a short-window expand override as soon as the window is tall again, so a later
    // shrink auto-collapses. Adjusting state during render is the recommended pattern here.
    if (!isShortWindow && expandedOnShortWindow) {
        setExpandedOnShortWindow(false);
    }

    const collapsed = !expandedOnShortWindow && (!!pref.hideStatusPanel || isShortWindow);

    const onToggle = React.useCallback(() => {
        if (isShortWindow) {
            if (collapsed) {
                if (pref.hideStatusPanel) {
                    services.viewPreferences.updatePreferences({appDetails: {...pref, hideStatusPanel: false}});
                }
                setExpandedOnShortWindow(true);
            } else {
                setExpandedOnShortWindow(false);
            }
            return;
        }
        services.viewPreferences.updatePreferences({appDetails: {...pref, hideStatusPanel: !pref.hideStatusPanel}});
    }, [collapsed, isShortWindow, pref]);

    return {collapsed, onToggle};
}
