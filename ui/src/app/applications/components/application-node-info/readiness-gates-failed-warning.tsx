import * as React from 'react';
import {selectPostfix} from '../utils';

import './readiness-gates-failed-warning.scss';

export interface ReadinessGatesFailedWarningProps {
    readinessGatesState: {
        nonExistingConditions: string[];
        failedConditions: string[];
    };
}

export const ReadinessGatesFailedWarning = ({readinessGatesState}: ReadinessGatesFailedWarningProps) => {
    if (readinessGatesState.failedConditions.length > 0 || readinessGatesState.nonExistingConditions.length > 0) {
        return (
            <div className='white-box white-box__readiness-gates-alert'>
                <h5>Readiness Gates Failing: </h5>
                <ul>
                    {readinessGatesState.failedConditions.length > 0 && (
                        <li>
                            The status of pod readiness gate{selectPostfix(readinessGatesState.failedConditions, '', 's')}{' '}
                            {readinessGatesState.failedConditions
                                .map(t => `"${t}"`)
                                .join(', ')
                                .trim()}{' '}
                            {selectPostfix(readinessGatesState.failedConditions, 'is', 'are')} False.
                        </li>
                    )}
                    {readinessGatesState.nonExistingConditions.length > 0 && (
                        <li>
                            Corresponding condition{selectPostfix(readinessGatesState.nonExistingConditions, '', 's')} of pod readiness gate{' '}
                            {readinessGatesState.nonExistingConditions
                                .map(t => `"${t}"`)
                                .join(', ')
                                .trim()}{' '}
                            do{selectPostfix(readinessGatesState.nonExistingConditions, 'es', '')} not exist.
                        </li>
                    )}
                </ul>
            </div>
        );
    }
    return null;
};
