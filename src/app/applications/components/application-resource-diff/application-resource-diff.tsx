import { Checkbox } from 'argo-ui';
import * as jsYaml from 'js-yaml';
import * as React from 'react';

const jsonDiffPatch = require('jsondiffpatch');

import { MonacoEditor } from '../../../shared/components';
import * as models from '../../../shared/models';

require('./application-resource-diff.scss');

export interface ApplicationComponentDiffProps {
    state: models.ResourceDiff;
}

export const ApplicationResourceDiff = (props: ApplicationComponentDiffProps) => {
    const [hideDefaultedFields, setHideDefaultedFields] = React.useState(true);
    const [inlineDiff, setInlineDiff] = React.useState(true);

    let live = props.state.liveState;
    if (hideDefaultedFields && live) {
        live = removeDefaultedFields(props.state.targetState, live);
    }

    const liveCopy = JSON.parse(JSON.stringify(live || {}));
    let target = null;
    if (props.state.targetState) {
        target = props.state.diff ? jsonDiffPatch.patch(liveCopy, JSON.parse(props.state.diff)) : liveCopy;
    }

    return (
        <div className='application-component-diff'>
            <div className='application-component-diff__checkboxs'>
                <Checkbox id='inlineDiff' checked={inlineDiff}
                        onChange={() => setInlineDiff(!inlineDiff)}/> <label htmlFor='inlineDiff'>
                    Inline Diff
                </label>  <Checkbox id='hideDefaultedFields' checked={hideDefaultedFields}
                        onChange={() => setHideDefaultedFields(!hideDefaultedFields)}/> <label htmlFor='hideDefaultedFields'>
                    Hide default fields
                </label>
            </div>
            <MonacoEditor diffEditor={{
                options: {
                    renderSideBySide: !inlineDiff,
                    readOnly: true,
                },
                modified: { text: target ? jsYaml.safeDump(target, {indent: 2 }) : '', language: 'yaml' },
                original: { text: live ? jsYaml.safeDump(live, {indent: 2 }) : '', language: 'yaml' },
                }}/>
        </div>
    );
};

function removeDefaultedFields(config: any, live: any): any {
    if (config instanceof Array) {
        const result = [];
        for (let i = 0; i < live.length; i++) {
            let v2 = live[i];
            if (config.length > i) {
                if (v2) {
                    v2 = removeDefaultedFields(config[i], v2);
                }
                result.push(v2);
            } else {
                result.push(v2);
            }
        }
        return result;
    } else if (config instanceof Object) {
        const result: any = {};
        for (const k of Object.keys(config)) {
            const v1 = config[k];
            if (live.hasOwnProperty(k)) {
                let v2 = live[k];
                if (v2) {
                    v2 = removeDefaultedFields(v1, v2);
                }
                result[k] = v2;
            }
        }
        return result;
    }
    return live;
}
