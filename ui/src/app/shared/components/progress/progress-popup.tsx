import {Popup} from 'argo-ui';
import * as React from 'react';
import {ProgressBar} from './progress-bar';
import './progress-popup.scss';

const Title = ({title}: {title: string}) => {
    return <span className='progress-popup__title'>{title}</span>;
};

const Footer = ({percentage, onClose}: {percentage: number; onClose: () => void}) => {
    return (
        <div className='progress-popup__footer'>
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
            <div className='progress-popup__content'>
                <ProgressBar percentage={percentage} />
            </div>
        </Popup>
    );
};
