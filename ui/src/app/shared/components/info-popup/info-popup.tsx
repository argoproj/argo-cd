import {Popup} from 'argo-ui';
import * as React from 'react';

const Title = ({title, onClose}: {title: JSX.Element; onClose: () => void}) => {
    return (
        <React.Fragment>
            {' '}
            <span>
                {title} <i className='argo-icon-close' onClick={onClose} />
            </span>
        </React.Fragment>
    );
};

export const InfoPopup = ({title, content, onClose}: {title: JSX.Element; content: JSX.Element; onClose: () => void}) => {
    return (
        <Popup title={<Title title={title} onClose={onClose} />}>
            <div>{content}</div>
        </Popup>
    );
};
