import * as React from 'react';
import {LogLoader} from './log-loader';
import {ToggleButton} from '../../../shared/components/toggle-button';

// ShowPreviousLogsToggleButton is a component that renders a toggle button that toggles previous logs.
export const ShowPreviousLogsToggleButton = ({
    setPreviousLogs,
    showPreviousLogs,
    loader
}: {
    setPreviousLogs: (value: boolean) => void;
    showPreviousLogs: boolean;
    loader: LogLoader;
}) => (
    <ToggleButton
        title='Show previous logs, i.e. logs from previous container restarts'
        onToggle={() => {
            setPreviousLogs(!showPreviousLogs);
            loader.reload();
        }}
        icon='angle-left'
        toggled={showPreviousLogs}
    />
);
