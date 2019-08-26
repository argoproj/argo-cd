import {Checkbox, DataLoader} from 'argo-ui';
import * as jsYaml from 'js-yaml';
import * as React from 'react';
import {Diff, Hunk, parseDiff} from 'react-diff-view';
import 'react-diff-view/style/index.css';
import {diffLines, formatLines} from 'unidiff';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';

const jsonDiffPatch = require('jsondiffpatch');

require('./application-resources-diff.scss');

export interface ApplicationResourcesDiffProps {
    states: models.ResourceDiff[];
}

export const ApplicationResourcesDiff = (props: ApplicationResourcesDiffProps) => (
    <DataLoader key='resource-diff' load={() => services.viewPreferences.getPreferences()}>
        {(pref) => {
            const diffText = props.states.map((state) => {
                let live = state.liveState;
                if (pref.appDetails.hideDefaultedFields && live) {
                    live = removeDefaultedFields(state.targetState, live);
                }

                const liveCopy = JSON.parse(JSON.stringify(live || {}));
                let target: any = null;
                if (state.targetState) {
                    target = state.diff ? jsonDiffPatch.patch(liveCopy, JSON.parse(state.diff)) : liveCopy;
                }

                return {
                    a: live ? jsYaml.safeDump(live, {indent: 2}) : '',
                    b: target ? jsYaml.safeDump(target, {indent: 2}) : '',
                    hook: state.hook,
                    // doubles as sort order
                    name: (state.group || '') + '/' + state.kind + '/' + state.namespace + '/' + state.name,
                };
            })
                .filter((i) => !i.hook)
                .filter((i) => i.a !== i.b)
                .map((i) => {
                const context = pref.appDetails.compactDiff ? 2 : Number.MAX_SAFE_INTEGER;
                // react-diff-view, awesome as it is, does not accept unidiff format, you must add a git header section
                return `diff --git a/${i.name} b/${i.name}
index 6829b8a2..4c565f1b 100644
${formatLines(diffLines(i.a, i.b), {context, aname: `a/${name}}`, bname: `b/${i.name}`})}`;
            }).join('\n');
            // assume that if you only have one file, we don't need the file path
            const whiteBox = props.states.length > 1 ? 'white-box' : '';
            const showPath = props.states.length > 1;
            const files = parseDiff(diffText);
            const viewType = pref.appDetails.inlineDiff ? 'unified' : 'split';
            return (
                <div className='application-resources-diff'>
                    <div className={whiteBox + ' application-resources-diff__checkboxes'}>
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
                                      appDetails: {
                                          ...pref.appDetails,
                                          inlineDiff: !pref.appDetails.inlineDiff,
                                      },
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
                    {files.sort((a: any, b: any) => a.newPath.localeCompare(b.newPath)).map((file: any) => (
                        <div key={file.newPath} className={whiteBox + ' application-component-diff__diff'}>
                            {showPath && (
                                <p className='application-resources-diff__diff__title'>{file.newPath}</p>
                            )}
                            <Diff viewType={viewType} diffType={file.type} hunks={file.hunks}>
                                {(hunks: any) => hunks.map((hunk: any) => (<Hunk key={hunk.content} hunk={hunk}/>))}
                            </Diff>
                        </div>
                    ))}
                </div>
            );
        }}
    </DataLoader>
);

function removeDefaultedFields(config: any, obj: any): any {
    if (config instanceof Array) {
        const result = [];
        for (let i = 0; i < obj.length; i++) {
            let v2 = obj[i];
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
            if (obj.hasOwnProperty(k)) {
                let v2 = obj[k];
                if (v2) {
                    v2 = removeDefaultedFields(v1, v2);
                }
                result[k] = v2;
            }
        }
        return result;
    }
    return obj;
}
