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

    let modified = props.state.liveState;
    if (hideDefaultedFields && modified) {
        modified = removeDefaultedFields(props.state.targetState, modified);
    }

    const modifiedCopy = JSON.parse(JSON.stringify(modified || {}));
    let original = null;
    if (props.state.targetState) {
        original = props.state.diff ? jsonDiffPatch.patch(modifiedCopy, JSON.parse(props.state.diff)) : modifiedCopy;
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
                original: { text: original ? jsYaml.safeDump(original, {indent: 2 }) : '', language: 'yaml' },
                modified: { text: modified ? jsYaml.safeDump(modified, {indent: 2 }) : '', language: 'yaml' },
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
