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
    const [prevHideStatusPanel, setPrevHideStatusPanel] = React.useState(pref.hideStatusPanel);

    // Drop a short-window expand override as soon as the window is tall again, so a later
    // shrink auto-collapses. Adjusting state during render is the recommended pattern here.
    if (!isShortWindow && expandedOnShortWindow) {
        setExpandedOnShortWindow(false);
    }

    // An explicit hide preference set in another window ends the local short-window expansion.
    // Track the previous value so a local expand is still possible while the preference stays hidden.
    if (pref.hideStatusPanel !== prevHideStatusPanel) {
        setPrevHideStatusPanel(pref.hideStatusPanel);
        if (pref.hideStatusPanel) {
            setExpandedOnShortWindow(false);
        }
    }

    const collapsed = !expandedOnShortWindow && (!!pref.hideStatusPanel || isShortWindow);

    const onToggle = React.useCallback(() => {
        if (isShortWindow) {
            if (collapsed) {
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
