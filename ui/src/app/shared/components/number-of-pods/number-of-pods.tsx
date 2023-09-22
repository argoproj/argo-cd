import * as React from 'react';

require('./number-of-pods.scss');

export const NumberOfPods = ({numberOfPods}: {numberOfPods: string}) => (
    <div className='number-of-pods'>
        <div className='number-of-pods__border'>
            <div className='number-of-pods__border__number'>{numberOfPods}</div>
            <div className='number-of-pods__border__label'>pod</div>
        </div>
    </div>
);
