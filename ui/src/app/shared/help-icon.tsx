import {Tooltip} from 'argo-ui/src/app/shared/components/tooltip/tooltip';
import * as React from 'react';

export const HelpIcon = ({title}: {title: React.ReactChild | React.ReactChild[]}) => (
    <Tooltip content={title}>
        <span style={{fontSize: 'smaller'}}>
            {' '}
            <i className='fa fa-question-circle help-tip' />
        </span>
    </Tooltip>
);
