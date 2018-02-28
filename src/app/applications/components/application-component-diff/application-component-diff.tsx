import { models } from 'argo-ui';
import * as React from 'react';

const jsonDiffPatch = require('jsondiffpatch');
require('./application-component-diff.scss');

export interface ApplicationComponentDiffProps {
    component: models.TypeMeta & { metadata: models.ObjectMeta };
    delta: any;
}

export const ApplicationComponentDiff = (props: ApplicationComponentDiffProps) => {
    const html = jsonDiffPatch.formatters.html.format(props.delta, props.component);
    return (
        <div className='application-component-diff' dangerouslySetInnerHTML={{__html: html}}/>
    );
};
