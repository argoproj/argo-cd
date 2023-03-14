import * as React from 'react';
import {ToggleButton} from '../../../shared/components/toggle-button';

// FollowToggleButton is a component that renders a button to toggle following logs.
export const FollowToggleButton = ({follow, setFollow}: {follow: boolean; setFollow: (value: boolean) => void}) => (
    <ToggleButton title='Follow logs, automatically showing new logs lines' onToggle={() => setFollow(!follow)} toggled={follow} icon='angles-down' />
);
