import {HelpIcon} from 'argo-ui';
import * as React from 'react';

export const RevisionHelpIcon = ({type, top, right}: {type: string; top?: string; right?: string}) => (
    <div style={{position: 'absolute', top: top === undefined ? '1em' : top, right: right === undefined ? '0.5em' : right}}>
        {type === 'helm' ? (
            <HelpIcon title='E.g. 1.2.0, 1.2.x, 1.x, or x' />
        ) : (
            <HelpIcon title='Branches, tags, commit hashes and symbolic refs are allowed. E.g. "master", "v1.2.0", "0a1b2c3", or "HEAD".' />
        )}
    </div>
);
