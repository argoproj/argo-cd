import {Checkbox, Tooltip} from "argo-ui";
import {services, ViewPreferences} from "../../../shared/services";
import * as React from "react";

export const WrapLinesToggleButton = ({prefs}: { prefs: ViewPreferences }) => <Tooltip content='Wrap Lines'>
    <button
        className={`argo-button argo-button--base-o`}
        onClick={() => {
            const wrap = prefs.appDetails.wrapLines;
            services.viewPreferences.updatePreferences({...prefs, appDetails: {...prefs.appDetails, wrapLines: !wrap}});
        }}>
        <Checkbox checked={prefs.appDetails.wrapLines}/>
        <i className='fa fa-paragraph'/>
    </button>
</Tooltip>