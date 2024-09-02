import * as React from 'react';
import {ToggleButton} from '../../../shared/components/toggle-button';

// ShowPreviousLogsToggleButton is a component that renders a toggle button that toggles previous logs.
export const ShowPreviousLogsToggleButton = ({setPreviousLogs, showPreviousLogs}: {setPreviousLogs: (value: boolean) => void; showPreviousLogs: boolean}) => (
    <ToggleButton
        title='Show previous logs, i.e. logs from previous container restarts'
        onToggle={() => setPreviousLogs(!showPreviousLogs)}
        icon='angle-left'
        toggled={showPreviousLogs}
    />
);
