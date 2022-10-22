import * as React from 'react';
import {ToggleButton} from '../../../shared/components/toggle-button';

export const FollowToggleButton = ({follow, setFollow}: {follow: boolean; setFollow: (value: boolean) => void}) => (
    <ToggleButton
        title='Follow logs'
        onToggle={() => {
            setFollow(!follow);
        }}
        toggled={follow}
        icon='turn-down'
    />
);
