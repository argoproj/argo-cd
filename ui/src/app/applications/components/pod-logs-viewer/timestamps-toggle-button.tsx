import * as React from 'react';
import {ToggleButton} from '../../../shared/components/toggle-button';

// TimestampsToggleButton is a component that renders a toggle button that toggles timestamps.
export const TimestampsToggleButton = ({
    timestamp,
    viewTimestamps,
    setViewTimestamps
}: {
    timestamp?: string;
    viewTimestamps: boolean;
    setViewTimestamps: (value: boolean) => void;
}) =>
    !timestamp && (
        <ToggleButton
            title='Show timestamps'
            onToggle={() => {
                setViewTimestamps(!viewTimestamps);
            }}
            toggled={viewTimestamps}
            icon='clock'
        />
    );
