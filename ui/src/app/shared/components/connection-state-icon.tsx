import * as React from 'react';
import * as models from '../models';

import {COLORS} from './colors';

export const ConnectionStateIcon = (props: {state: models.ConnectionState}) => {
    let className = '';
    let color = '';

    switch (props.state.status) {
        case models.ConnectionStatuses.Successful:
            className = 'fa fa-check-circle';
            color = COLORS.connection_status.successful;
            break;
        case models.ConnectionStatuses.Failed:
            className = 'fa fa-times';
            color = COLORS.connection_status.failed;
            break;
        case models.ConnectionStatuses.Unknown:
            className = 'fa fa-exclamation-circle';
            color = COLORS.connection_status.unknown;
            break;
    }
    return <i title={props.state.message || props.state.status} className={className} style={{color}} />;
};
