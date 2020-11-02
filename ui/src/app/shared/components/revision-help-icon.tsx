import {HelpIcon} from 'argo-ui';
import * as React from 'react';

export const RevisionHelpIcon = ({type, top}: {type: string; top?: string}) => (
    <div style={{position: 'absolute', top: top === undefined ? '1em' : top, right: '0.5em'}}>
        {type === 'helm' ? (
            <HelpIcon title='E.g. 1.2.0, 1.2.*, 1.*, or *' />
        ) : (
            <HelpIcon title='Branches, tags, commit hashes and symbolic refs are allowed. E.g. "master", "v1.2.0", "0a1b2c3", or "HEAD".' />
        )}
    </div>
);
