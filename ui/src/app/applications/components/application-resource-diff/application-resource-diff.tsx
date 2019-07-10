import {Checkbox, DataLoader} from 'argo-ui';
import * as jsYaml from 'js-yaml';
import * as React from 'react';
import {Diff, Hunk, parseDiff} from 'react-diff-view';
import 'react-diff-view/style/index.css';
import {diffLines, formatLines} from 'unidiff';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';

const jsonDiffPatch = require('jsondiffpatch');

require('./application-resource-diff.scss');

export interface ApplicationComponentDiffProps {
    state: models.ResourceDiff;
}

export const ApplicationResourceDiff = (props: ApplicationComponentDiffProps) => (
    <DataLoader load={() => services.viewPreferences.getPreferences()}>
        {(pref) => {
            let live = props.state.liveState;
            if (pref.appDetails.hideDefaultedFields && live) {
                live = removeDefaultedFields(props.state.targetState, live);
            }

            const liveCopy = JSON.parse(JSON.stringify(live || {}));
            let target: any = null;
            if (props.state.targetState) {
                target = props.state.diff ? jsonDiffPatch.patch(liveCopy, JSON.parse(props.state.diff)) : liveCopy;
            }

            const a = live ? jsYaml.safeDump(live, {indent: 2}) : '';
            const b = target ? jsYaml.safeDump(target, {indent: 2}) : '';
            const context = pref.appDetails.compactDiff ? 2 : Number.MAX_SAFE_INTEGER;
            const diffText = formatLines(diffLines(a, b), {context});
            const files = parseDiff(diffText, {nearbySequences: 'zip'});
            const viewType = pref.appDetails.inlineDiff ? 'unified' : 'split';
            return (
                <div className='application-component-diff'>
                    <div className='application-component-diff__checkboxes'>
                        <Checkbox id='compactDiff' checked={pref.appDetails.compactDiff}
                                  onChange={() => services.viewPreferences.updatePreferences({
                                      appDetails: {
                                          ...pref.appDetails,
                                          compactDiff: !pref.appDetails.compactDiff,
                                      },
                                  })}/>
                        <label htmlFor='compactDiff'>Compact diff</label>
                        <Checkbox id='inlineDiff' checked={pref.appDetails.inlineDiff}
                                  onChange={() => services.viewPreferences.updatePreferences({
                                      appDetails: {...pref.appDetails, inlineDiff: !pref.appDetails.inlineDiff},
                                  })}/>
                        <label htmlFor='inlineDiff'>Inline Diff</label>
                        <Checkbox id='hideDefaultedFields' checked={pref.appDetails.hideDefaultedFields}
                                  onChange={() => services.viewPreferences.updatePreferences({
                                      appDetails: {
                                          ...pref.appDetails,
                                          hideDefaultedFields: !pref.appDetails.hideDefaultedFields,
                                      },
                                  })}/>
                        <label htmlFor='hideDefaultedFields'>Hide default fields</label>
                    </div>
                    <div className='application-component-diff__diff'>
                        {files.map((file: any) => (
                            <Diff key={file.oldRevision + '-' + file.newRevision} viewType={viewType}
                                  diffType={file.type} hunks={file.hunks}>
                                {(hunks: any) => hunks.map((hunk: any) => (<Hunk key={hunk.content} hunk={hunk}/>))}
                            </Diff>
                        ))
                        }
                    </div>
                </div>
            );
        }}
    </DataLoader>
);

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
