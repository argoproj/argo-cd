import * as React from 'react';
import * as models from '../../../shared/models';

import './application-retry-view.scss';

function buildRetryOptionView(label: string, data: string | number) {
    const result = data || 'not installed';

    return (
        <div className='application-retry-option-view-list__item'>
            {label} - {result}
        </div>
    );
}

const retryOptionsView: Array<(initData: models.RetryStrategy) => React.ReactNode> = [
    initData => buildRetryOptionView('Limit', initData?.limit),
    initData => buildRetryOptionView('Duration', initData?.backoff?.duration),
    initData => buildRetryOptionView('Max Duration', initData?.backoff?.maxDuration),
    initData => buildRetryOptionView('Factor', initData?.backoff?.factor)
];

export const ApplicationRetryView = ({initValues}: {initValues?: models.RetryStrategy}) => {
    const result = !initValues ? 'Retry disabled' : retryOptionsView.map(render => render(initValues));
    return <div className='application-retry-option-view-list'>{result}</div>;
};
