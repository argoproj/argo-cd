import {Tooltip} from 'argo-ui';
import * as React from 'react';
import {ApplicationSource as ApplicationSourceType} from '../../../shared/models';

import './applications-source.scss';

export const ApplicationsSource = ({source}: {source: ApplicationSourceType}) => {
    const sourceString = `${source.repoURL}/${source.path || source.chart}`;
    return (
        <Tooltip content={sourceString}>
            <div className='application-source'>{sourceString}</div>
        </Tooltip>
    );
};
