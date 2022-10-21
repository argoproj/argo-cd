import {Tooltip} from "argo-ui";
import {services, ViewPreferences} from "../../../shared/services";
import * as React from "react";

export const DarkModeToggleButton = ({prefs}: { prefs: ViewPreferences }) => <Tooltip
    content={prefs.appDetails.darkMode ? 'Light Mode' : 'Dark Mode'}>
    <button
        className='argo-button argo-button--base-o'
        onClick={() => {
            const inverted = prefs.appDetails.darkMode;
            services.viewPreferences.updatePreferences({
                ...prefs,
                appDetails: {...prefs.appDetails, darkMode: !inverted}
            });
        }}>
        {prefs.appDetails.darkMode ? <i className='fa fa-sun'/> : <i className='fa fa-moon'/>}
    </button>
</Tooltip>