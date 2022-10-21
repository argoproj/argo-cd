import {services, ViewPreferences} from '../../../shared/services';
import * as React from 'react';
import {LogLoader} from './log-loader';
import {ToggleButton} from '../../../shared/components/toggle-button';

export const FollowToggleButton = ({
    prefs,
    page,
    setPage,
    loader
}: {
    page: {number: number};
    setPage: (page: {number: number; untilTimes: []}) => void;
    prefs: ViewPreferences;
    loader: LogLoader;
}) => (
    <ToggleButton
        title='Follow logs'
        disabled={page.number > 0}
        onToggle={() => {
            if (page.number > 0) {
                return;
            }
            const follow = !prefs.appDetails.followLogs;
            services.viewPreferences.updatePreferences({...prefs, appDetails: {...prefs.appDetails, followLogs: follow}});
            if (follow) {
                setPage({number: 0, untilTimes: []});
            }
            loader.reload();
        }}
        toggled={prefs.appDetails.followLogs}
        icon='turn-down'
    />
);
