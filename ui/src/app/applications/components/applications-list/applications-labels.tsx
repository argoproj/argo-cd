import * as React from 'react';
import {Tooltip} from 'argo-ui';
import {Application} from '../../../shared/models';

require('./applications-labels.scss');

export const ApplicationsLabels = ({app}: {app: Application}) => {
    const labels = (
        <>
            <span className='application-labels__item'>{app.spec.source.targetRevision || 'HEAD'}</span>
            {Object.keys(app.metadata.labels || {}).map(label => (
                <span className='application-labels__item' key={label}>{`${label}=${app.metadata.labels[label]}`}</span>
            ))}
        </>
    );

    return (
        <Tooltip
            popperOptions={{
                modifiers: {
                    preventOverflow: {
                        enabled: true
                    }
                }
            }}
            placement='auto-start'
            content={<div className='application-labels-tooltip'>{labels}</div>}>
            <div className='application-labels'>{labels}</div>
        </Tooltip>
    );
};
