import * as React from 'react';

import * as models from '../../shared/models';

const ARGO_SUCCESS_COLOR = '#18BE94';
const ARGO_FAILED_COLOR = '#E96D76';
const ARGO_RUNNING_COLOR = '#0DADEA';

export const ComparisonStatusIcon = ({status}: { status: models.ComparisonStatus }) => {
    let className = '';
    let color = '';

    switch (status) {
        case models.ComparisonStatuses.Synched:
            className = 'fa fa-check-circle';
            color = ARGO_SUCCESS_COLOR;
            break;
        case models.ComparisonStatuses.OutOfSync:
            className = 'fa fa-times';
            color = ARGO_FAILED_COLOR;
            break;
        case models.ComparisonStatuses.Error:
            className = 'fa fa-exclamation-circle';
            color = ARGO_FAILED_COLOR;
            break;
        case models.ComparisonStatuses.Unknown:
            className = 'fa fa-circle-o-notch status-icon--running status-icon--spin';
            color = ARGO_RUNNING_COLOR;
            break;
    }
    return <i className={className} style={{ color }} />;
};
