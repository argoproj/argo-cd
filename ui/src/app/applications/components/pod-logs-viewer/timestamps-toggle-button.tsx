import * as React from 'react';
import {ToggleButton} from '../../../shared/components/toggle-button';

export const TimestampsToggleButton = ({
    timestamp,
    viewTimestamps,
    setViewTimestamps,
    viewPodNames,
    setViewPodNames
}: {
    timestamp?: string;
    viewTimestamps: boolean;
    setViewTimestamps: (value: boolean) => void;
    viewPodNames: boolean;
    setViewPodNames: (value: boolean) => void;
}) =>
    !timestamp && (
        <ToggleButton
            title='Show timestamps'
            onToggle={() => {
                setViewTimestamps(!viewTimestamps);
                if (viewPodNames) {
                    setViewPodNames(false);
                }
            }}
            toggled={viewTimestamps}
            icon='clock'
        />
    );
