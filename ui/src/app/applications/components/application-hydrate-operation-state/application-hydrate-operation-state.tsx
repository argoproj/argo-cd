import {Duration, Ticker} from 'argo-ui';
import * as moment from 'moment';
import * as PropTypes from 'prop-types';
import * as React from 'react';

import {Revision, Timestamp} from '../../../shared/components';
import * as models from '../../../shared/models';

import './application-hydrate-operation-state.scss';

interface Props {
    hydrateOperationState: models.HydrateOperation;
}

export const ApplicationHydrateOperationState: React.FunctionComponent<Props> = ({hydrateOperationState}) => {
    const operationAttributes = [
        {title: 'PHASE', value: hydrateOperationState.phase},
        ...(hydrateOperationState.message ? [{title: 'MESSAGE', value: hydrateOperationState.message}] : []),
        {title: 'STARTED AT', value: <Timestamp date={hydrateOperationState.startedAt} />},
        {
            title: 'DURATION',
            value: (
                <Ticker>
                    {time => (
                        <Duration
                            durationS={
                                ((hydrateOperationState.finishedAt && moment(hydrateOperationState.finishedAt)) || moment(time)).diff(moment(hydrateOperationState.startedAt)) /
                                1000
                            }
                        />
                    )}
                </Ticker>
            )
        }
    ];

    if (hydrateOperationState.finishedAt && hydrateOperationState.phase !== 'Hydrating') {
        operationAttributes.push({title: 'FINISHED AT', value: <Timestamp date={hydrateOperationState.finishedAt} />});
    }
    operationAttributes.push({
        title: 'DRY REVISION',
        value: (
            <div>
                <Revision repoUrl={hydrateOperationState.sourceHydrator.drySource.repoURL} revision={hydrateOperationState.drySHA} />
            </div>
        )
    });
    if (hydrateOperationState.finishedAt) {
        operationAttributes.push({
            title: 'HYDRATED REVISION',
            value: (
                <div>
                    <Revision repoUrl={hydrateOperationState.sourceHydrator.drySource.repoURL} revision={hydrateOperationState.hydratedSHA} />
                </div>
            )
        });
    }
    return (
        <div>
            <div className='white-box'>
                <div className='white-box__details'>
                    {operationAttributes.map(attr => (
                        <div className='row white-box__details-row' key={attr.title}>
                            <div className='columns small-3'>{attr.title}</div>
                            <div className='columns small-9'>{attr.value}</div>
                        </div>
                    ))}
                </div>
            </div>
        </div>
    );
};

ApplicationHydrateOperationState.contextTypes = {
    apis: PropTypes.object
};
