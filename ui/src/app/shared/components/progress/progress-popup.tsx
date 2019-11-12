import {Popup} from 'argo-ui/src/app/shared/components/popup/popup';
import * as React from 'react';
import {ProgressBar} from './progress-bar';

export const ProgressPopup = ({title, percentage, onClose}: {title: string; percentage: number; onClose: () => void}) => {
    return (
        <Popup
            title={
                <span>
                    {title} <i className='argo-icon-close' onClick={() => onClose()} />
                </span>
            }>
            <ProgressBar percentage={percentage} />
        </Popup>
    );
};
