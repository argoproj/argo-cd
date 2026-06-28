import {ToggleButton} from '../../../shared/components/toggle-button';
import * as React from 'react';

export const MatchCaseToggleButton = ({matchCase, setMatchCase}: {matchCase: boolean; setMatchCase: (matchCase: boolean) => void}) => {
    return (
        <ToggleButton title='Match Case' onToggle={() => setMatchCase(!matchCase)} toggled={matchCase}>
            <span style={{textTransform: 'none'}}>Aa</span>
        </ToggleButton>
    );
};
