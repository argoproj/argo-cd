import * as React from 'react';
import {selectPostfix} from '../utils';

import './readiness-gates-not-passed-warning.scss';

export interface ReadinessGatesNotPassedWarningProps {
    readinessGatesState: {
        nonExistingConditions: string[];
        notPassedConditions: string[];
    };
}

export const ReadinessGatesNotPassedWarning = ({readinessGatesState}: ReadinessGatesNotPassedWarningProps) => {
    if (readinessGatesState.notPassedConditions.length > 0 || readinessGatesState.nonExistingConditions.length > 0) {
        return (
            <div className='white-box white-box__readiness-gates-alert'>
                <h5>Readiness Gates Not Passing: </h5>
                <ul>
                    {readinessGatesState.notPassedConditions.length > 0 && (
                        <li>
                            The status of pod readiness gate{selectPostfix(readinessGatesState.notPassedConditions, '', 's')}{' '}
                            {readinessGatesState.notPassedConditions
                                .map(t => `"${t}"`)
                                .join(', ')
                                .trim()}{' '}
                            {selectPostfix(readinessGatesState.notPassedConditions, 'is', 'are')} False.
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
