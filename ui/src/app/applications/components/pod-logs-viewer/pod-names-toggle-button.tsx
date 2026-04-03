import * as React from 'react';
import {ToggleButton} from '../../../shared/components/toggle-button';

// PodNamesToggleButton is a component that renders a toggle button that toggles pod names.
export const PodNamesToggleButton = ({viewPodNames, setViewPodNames}: {viewPodNames: boolean; setViewPodNames: (value: boolean) => void}) => (
    <ToggleButton title='Show pod names' onToggle={() => setViewPodNames(!viewPodNames)} toggled={viewPodNames} icon='box' />
);
