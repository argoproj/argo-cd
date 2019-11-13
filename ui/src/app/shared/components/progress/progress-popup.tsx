import {Popup} from 'argo-ui/src/app/shared/components/popup/popup';
import * as React from 'react';
import {ProgressBar} from './progress-bar';

const Title = ({title, onClose}: {title: string; onClose: () => void}) => {
    return (
        <React.Fragment>
            {title} <i className='argo-icon-close' onClick={() => onClose()} />
        </React.Fragment>
    );
};

const Footer = ({percentage, onClose}: {percentage: number; onClose: () => void}) => {
    return (
        <div style={{textAlign: 'right'}}>
            {percentage >= 100 && (
                <button className='argo-button argo-button--base-o' onClick={() => onClose()}>
                    Close
                </button>
            )}
        </div>
    );
};

export const ProgressPopup = ({title, percentage, onClose}: {title: string; percentage: number; onClose: () => void}) => {
    return (
        <Popup title={<Title title={title} onClose={onClose} />} footer={<Footer percentage={percentage} onClose={onClose} />}>
            <ProgressBar percentage={percentage} />
        </Popup>
    );
};
