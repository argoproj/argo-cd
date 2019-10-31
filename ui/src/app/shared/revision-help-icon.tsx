import * as React from 'react';
import {HelpIcon} from './help-icon';

export const RevisionHelpIcon = ({type}: {type: string}) =>
    type === 'helm' ? <HelpIcon title='E.g. 1.2.0, 1.2.*, 1.*, or *' /> : <HelpIcon title='E.g. v1.2.0, v1.2, v1, dev, e2e, master, or HEAD' />;
