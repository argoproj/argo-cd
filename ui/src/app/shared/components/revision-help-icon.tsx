import {HelpIcon} from 'argo-ui';
import * as React from 'react';

export const RevisionHelpIcon = ({type, top}: {type: string; top?: string}) => (
    <div style={{position: 'absolute', top: top === undefined ? '1em' : top, right: '0.5em'}}>
        {type === 'helm' ? <HelpIcon title='E.g. 1.2.0, 1.2.*, 1.*, or *' /> : <HelpIcon title='E.g. v1.2.0, v1.2, v1, dev, e2e, master, or HEAD' />}
    </div>
);
