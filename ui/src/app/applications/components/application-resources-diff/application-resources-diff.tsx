import {Checkbox, DataLoader} from 'argo-ui';
import * as jsYaml from 'js-yaml';
import * as React from 'react';
import {parseDiff} from 'react-diff-view';
import 'react-diff-view/style/index.css';
import {diffLines, formatLines} from 'unidiff';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {IndividualDiffSection} from './individual-diff-section';

import './application-resources-diff.scss';

export interface ApplicationResourcesDiffProps {
    states: models.ResourceDiff[];
}

export const ApplicationResourcesDiff = (props: ApplicationResourcesDiffProps) => (
    <DataLoader key='resource-diff' load={() => services.viewPreferences.getPreferences()}>
        {pref => {
            const diffText = props.states
                .map(state => {
                    return {
                        a: state.normalizedLiveState ? jsYaml.dump(state.normalizedLiveState, {indent: 2}) : '',
                        b: state.predictedLiveState ? jsYaml.dump(state.predictedLiveState, {indent: 2}) : '',
                        hook: state.hook,
                        // doubles as sort order
                        name: (state.group || '') + '/' + state.kind + '/' + (state.namespace ? state.namespace + '/' : '') + state.name
                    };
                })
                .filter(i => !i.hook)
                .filter(i => i.a !== i.b)
                .map(i => {
                    const context = pref.appDetails.compactDiff ? 2 : Number.MAX_SAFE_INTEGER;
                    // react-diff-view, awesome as it is, does not accept unidiff format, you must add a git header section
                    return `diff --git a/${i.name} b/${i.name}
index 6829b8a2..4c565f1b 100644
${formatLines(diffLines(i.a, i.b), {context, aname: `a/${name}}`, bname: `b/${i.name}`})}`;
                })
                .join('\n');
            // assume that if you only have one file, we don't need the file path
            const whiteBox = props.states.length > 1 ? 'white-box' : '';
            const showPath = props.states.length > 1;
            const files = parseDiff(diffText);
            const viewType = pref.appDetails.inlineDiff ? 'unified' : 'split';
            return (
                <div className='application-resources-diff'>
                    <div className={whiteBox + ' application-resources-diff__checkboxes'}>
                        <Checkbox
                            id='compactDiff'
                            checked={pref.appDetails.compactDiff}
                            onChange={() =>
                                services.viewPreferences.updatePreferences({
                                    appDetails: {
                                        ...pref.appDetails,
                                        compactDiff: !pref.appDetails.compactDiff
                                    }
                                })
                            }
                        />
                        <label htmlFor='compactDiff'>Compact diff</label>
                        <Checkbox
                            id='inlineDiff'
                            checked={pref.appDetails.inlineDiff}
                            onChange={() =>
                                services.viewPreferences.updatePreferences({
                                    appDetails: {
                                        ...pref.appDetails,
                                        inlineDiff: !pref.appDetails.inlineDiff
                                    }
                                })
                            }
                        />
                        <label htmlFor='inlineDiff'>Inline diff</label>
                    </div>
                    {files
                        .sort((a: any, b: any) => a.newPath.localeCompare(b.newPath))
                        .map((file: any) => (
                            <IndividualDiffSection key={file.newPath} file={file} showPath={showPath} whiteBox={whiteBox} viewType={viewType} />
                        ))}
                </div>
            );
        }}
    </DataLoader>
);
