import { models } from 'argo-ui';
import * as React from 'react';

const jsonDiffPatch = require('jsondiffpatch');
require('./application-resource-diff.scss');

export interface ApplicationComponentDiffProps {
    liveState: models.TypeMeta & { metadata: models.ObjectMeta };
    targetState: models.TypeMeta & { metadata: models.ObjectMeta };
}

export const ApplicationResourceDiff = (props: ApplicationComponentDiffProps) => {
    const delta = jsonDiffPatch.diff(props.targetState || {}, props.liveState || {});
    const html = jsonDiffPatch.formatters.html.format(delta, props.targetState);
    return (
        <div className='application-component-diff' dangerouslySetInnerHTML={{__html: html}}/>
    );
};
