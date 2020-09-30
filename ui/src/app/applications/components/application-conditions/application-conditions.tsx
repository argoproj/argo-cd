import * as React from 'react';

import {Timestamp} from '../../../shared/components';
import * as models from '../../../shared/models';
import {getConditionCategory} from '../utils';

require('./application-conditions.scss');

export const ApplicationConditions = ({conditions}: {conditions: models.ApplicationCondition[]}) => {
    return (
        <div className='application-conditions'>
            <h4>Application conditions</h4>
            {(conditions.length === 0 && <p>Application is healthy</p>) || (
                <div className='argo-table-list'>
                    {conditions.map((condition, index) => (
                        <div className={`argo-table-list__row application-conditions__condition application-conditions__condition--${getConditionCategory(condition)}`} key={index}>
                            <div className='row'>
                                <div className='columns small-2'>{condition.type}</div>
                                <div className='columns small-7'>{condition.message}</div>
                                <div className='columns small-3'>
                                    <Timestamp date={condition.lastTransitionTime} />
                                </div>
                            </div>
                        </div>
                    ))}
                </div>
            )}
        </div>
    );
};
