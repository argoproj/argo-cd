import * as React from 'react';
import * as models from '../models';

import { ARGO_FAILED_COLOR, ARGO_RUNNING_COLOR, ARGO_SUCCESS_COLOR } from './colors';

export const ConnectionStateIcon = (props: { state: models.ConnectionState }) => {
    let className = '';
    let color = '';

    switch (props.state.status) {
        case models.ConnectionStatuses.Successful:
            className = 'fa fa-check-circle';
            color = ARGO_SUCCESS_COLOR;
            break;
        case models.ConnectionStatuses.Failed:
            className = 'fa fa-times';
            color = ARGO_FAILED_COLOR;
            break;
        case models.ConnectionStatuses.Unknown:
            className = 'fa fa-exclamation-circle';
            color = ARGO_RUNNING_COLOR;
            break;
    }
    return <i title={props.state.message || props.state.status} className={className} style={{ color }} />;
};
