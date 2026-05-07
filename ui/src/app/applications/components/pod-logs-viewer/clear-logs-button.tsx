import * as React from 'react';
import {Button} from '../../../shared/components/button';

interface ClearLogsButtonProps {
    disabled?: boolean;
    onClear: () => void;
}

export const ClearLogsButton = ({disabled, onClear}: ClearLogsButtonProps) => <Button title='Clear displayed logs' icon='eraser' onClick={onClear} disabled={disabled} />;
