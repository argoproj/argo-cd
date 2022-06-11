import {Popup} from 'argo-ui';
import * as React from 'react';
import {ProgressBar} from './progress-bar';

const Title = ({title}: {title: string}) => {
    return <React.Fragment>{title}</React.Fragment>;
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
        <Popup title={<Title title={title} />} footer={<Footer percentage={percentage} onClose={onClose} />}>
            <ProgressBar percentage={percentage} />
        </Popup>
    );
};
