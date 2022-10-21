import {Checkbox, Tooltip} from "argo-ui";
import * as React from "react";
import {LogLoader} from "./log-loader";

export const ShowPreviousLogsToggleButton = ({setPreviousLogs, showPreviousLogs, loader}: {
    setPreviousLogs: (value: boolean) => void,
    showPreviousLogs: boolean,
    loader: LogLoader,
}) => <Tooltip content='Show previous logs'>
    <button
        className={`argo-button argo-button--base-o`}
        onClick={() => {
            setPreviousLogs(!showPreviousLogs);
            loader.reload();
        }}>
        <Checkbox checked={showPreviousLogs}/>
        <i className='fa fa-backward'/>
    </button>
</Tooltip>