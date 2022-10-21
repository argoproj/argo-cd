import {Checkbox, Tooltip} from "argo-ui";
import * as React from "react";

export const TimestampsToggleButton = ({
                                           timestamp,
                                           viewTimestamps,
                                           setViewTimestamps,
                                           viewPodNames,
                                           setViewPodNames
                                       }: { timestamp?: string, viewTimestamps: boolean, setViewTimestamps: (value: boolean) => void, viewPodNames: boolean, setViewPodNames: (value: boolean) => void }) =>
    !timestamp && (
        <Tooltip content={viewTimestamps ? 'Hide timestamps' : 'Show timestamps'}>
            <button
                className={'argo-button argo-button--base-o'}
                onClick={() => {
                    setViewTimestamps(!viewTimestamps);
                    if (viewPodNames) {
                        setViewPodNames(false);
                    }
                }}>
                <Checkbox checked={viewTimestamps}/>
                <i className='fa fa-clock'/>
            </button>
        </Tooltip>
    )
