import * as React from 'react';
import {ToggleButton} from '../../../shared/components/toggle-button';

export const AutoScrollButton = ({scrollToBottom, setScrollToBottom}: {scrollToBottom: boolean; setScrollToBottom: (value: boolean) => void}) => {
    return (
        <ToggleButton
            icon='circle-down'
            onToggle={() => setScrollToBottom(!scrollToBottom)}
            toggled={scrollToBottom}
            beat={scrollToBottom}
            title='Automatically scroll to the bottom when new content appears'
        />
    );
};
