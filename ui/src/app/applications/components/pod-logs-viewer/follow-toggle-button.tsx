import {Checkbox, Tooltip} from "argo-ui";
import * as classNames from "classnames";
import {services, ViewPreferences} from "../../../shared/services";
import * as React from "react";
import {LogLoader} from "./log-loader";

export const FollowToggleButton = ({prefs, page, setPage, loader}:{page:{number:number},
    setPage:(page:{number:number,untilTimes:[]}) => void,
    prefs:ViewPreferences,loader:LogLoader}) =>       <Tooltip content='Follow'>
    <button
        className={classNames(`argo-button argo-button--base-o`, {
            disabled: page.number > 0
        })}
        onClick={() => {
            if (page.number > 0) {
                return;
            }
            const follow = !prefs.appDetails.followLogs;
            services.viewPreferences.updatePreferences({...prefs, appDetails: {...prefs.appDetails, followLogs: follow}});
            if (follow) {
                setPage({number: 0, untilTimes: []});
            }
            loader.reload();
        }}>
        <Checkbox checked={prefs.appDetails.followLogs} />
        <i className='fa fa-arrow-right' />
    </button>
</Tooltip>