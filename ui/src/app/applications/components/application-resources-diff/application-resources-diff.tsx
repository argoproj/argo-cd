import {Checkbox, DataLoader} from 'argo-ui';
import * as jsYaml from 'js-yaml';
import * as React from 'react';
import {parseDiff} from 'react-diff-view';
import {File} from 'gitdiff-parser';
import 'react-diff-view/style/index.css';
import {diffLines, formatLines} from 'unidiff';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {IndividualDiffSection} from './individual-diff-section';

import './application-resources-diff.scss';
import {cleanState} from '../utils';
import {cloneDeep} from 'lodash-es';

export interface ApplicationResourcesDiffProps {
    states: models.ResourceDiff[];
    singleResource?: boolean;
}

// this is a copy of the File type from gitdiff-parser, but with an additional field
// to store the old source of the file (used for the "show more" functionality of the diff viewer)
export interface FileWithSource extends File {
    oldSource: string | null;
}

export const ApplicationResourcesDiff = (props: ApplicationResourcesDiffProps) => (
    <DataLoader key='resource-diff' load={() => services.viewPreferences.getPreferences()}>
        {pref => {
            // hide the extra info if we are in "single resource" mode (e.g. the node-info view)
            const whiteBox = props.singleResource ? '' : 'white-box';
            const showPath = !props.singleResource;
            const showHeadings = !props.singleResource;

            const filesAdded: any[] = [];
            const filesDeleted: any[] = [];
            const filesDeletedNoPrune: any[] = [];
            const filesModified: any[] = [];

            props.states
                .map(state => {
                    let normalizedLiveState = state.normalizedLiveState;
                    let predictedLiveState = state.predictedLiveState;
                    if (pref.appDetails.hideSystemFields) {
                        // if we are removing fields, we need to clone the object
                        normalizedLiveState = cloneDeep(normalizedLiveState);
                        predictedLiveState = cloneDeep(predictedLiveState);
                        cleanState(normalizedLiveState, {hideSystemFields: pref.appDetails.hideSystemFields});
                        cleanState(predictedLiveState, {hideSystemFields: pref.appDetails.hideSystemFields});
                    }
                    return {
                        // we remove the trailing newline from the yaml, as otherwise it always shows in the diff
                        a: normalizedLiveState ? jsYaml.dump(normalizedLiveState, {indent: 2, lineWidth: -1}).replace(/\n$/, '') : '',
                        b: predictedLiveState ? jsYaml.dump(predictedLiveState, {indent: 2, lineWidth: -1}).replace(/\n$/, '') : '',
                        hook: state.hook,
                        requiresPruning: state.requiresPruning,
                        pruningDisabled: state.pruningDisabled,
                        ignoreExtraneous: state.ignoreExtraneous,
                        // doubles as sort order
                        name: (state.group || '') + '/' + state.kind + '/' + (state.namespace ? state.namespace + '/' : '') + state.name
                    };
                })
                .filter(i => !i.hook)
                // NOTE: in the diff, we want to see resources that will be pruned OR are causing the app to be out of sync.
                //       so, we only exclude resources with pruning disabled AND the IgnoreExtraneous annotation.
                .filter(i => !(i.requiresPruning && i.pruningDisabled && i.ignoreExtraneous))
                .filter(i => i.a !== i.b)
                .sort((a, b) => a.name.localeCompare(b.name))
                .forEach(i => {
                    // using '/dev/null' as the name for old/new indicates that the file is added/deleted
                    const aName = i.a === '' ? `/dev/null` : `a/${i.name}`;
                    const bName = i.b === '' ? `/dev/null` : `b/${i.name}`;

                    // we show 3 lines of context around each change by default
                    // but the user can click "show more" to see the full diff
                    const contextLines = pref.appDetails.compactDiff ? 3 : Number.MAX_SAFE_INTEGER;

                    // calculate the diff in unified format
                    const unifiedDiff = formatLines(diffLines(i.a, i.b), {context: contextLines, aname: aName, bname: bName});

                    // construct the equivalent of a `git diff` command
                    // NOTE: the react-diff-view library requires the `diff --git` line to be present
                    const gitDiffLines = [`diff --git ${aName} ${bName}`, `index 0000000..0000001 100644`, unifiedDiff];
                    const gitDiff = gitDiffLines.join('\n');

                    // setting 'nearbySequences' to 'zip' will interleave the lines of nearby
                    // changes, which is a more readable format for the user when not in "inline" mode
                    const [file] = parseDiff(gitDiff, {nearbySequences: pref.appDetails.inlineDiff ? undefined : 'zip'});

                    // add the old source to the file object so we can display it
                    const fileSource: FileWithSource = {...file, oldSource: i.a ? i.a : null};

                    if (fileSource.type === 'add') {
                        filesAdded.push(fileSource);
                    } else if (file.type === 'delete') {
                        if (i.pruningDisabled) {
                            filesDeletedNoPrune.push(fileSource);
                        } else {
                            filesDeleted.push(fileSource);
                        }
                    } else if (fileSource.type === 'modify') {
                        filesModified.push(fileSource);
                    } else {
                        console.error('Unknown file diff type', fileSource);
                    }
                });

            if (filesAdded.length === 0 && filesDeleted.length === 0 && filesModified.length === 0 && filesDeletedNoPrune.length === 0) {
                return <div className={whiteBox}>No differences</div>;
            }

            const viewType = pref.appDetails.inlineDiff ? 'unified' : 'split';
            return (
                <div className='application-resources-diff'>
                    <div className={whiteBox + ' application-resources-diff__checkboxes'}>
                        <Checkbox
                            id='hideSystemFields'
                            checked={pref.appDetails.hideSystemFields}
                            onChange={() =>
                                services.viewPreferences.updatePreferences({
                                    appDetails: {
                                        ...pref.appDetails,
                                        hideSystemFields: !pref.appDetails.hideSystemFields
                                    }
                                })
                            }
                        />
                        <label htmlFor='hideSystemFields'>Hide System Fields</label>
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
                    {showHeadings && filesAdded.length > 0 && (
                        <div className={whiteBox + ' application-resources-diff__heading'}>
                            <b>Added Resources</b>
                        </div>
                    )}
                    {filesAdded.map((file: any) => (
                        <IndividualDiffSection key={file.newPath} resourceName={file.newPath} file={file} showPath={showPath} whiteBox={whiteBox} viewType={viewType} />
                    ))}
                    {showHeadings && filesModified.length > 0 && (
                        <div className={whiteBox + ' application-resources-diff__heading'}>
                            <b>Modified Resources</b>
                        </div>
                    )}
                    {filesModified.map((file: any) => (
                        <IndividualDiffSection key={file.oldPath} resourceName={file.oldPath} file={file} showPath={showPath} whiteBox={whiteBox} viewType={viewType} />
                    ))}
                    {showHeadings && filesDeleted.length > 0 && (
                        <div className={whiteBox + ' application-resources-diff__heading'}>
                            <b>Removed Resources</b>
                        </div>
                    )}
                    {filesDeleted.map((file: any) => (
                        <IndividualDiffSection key={file.oldPath} resourceName={file.oldPath} file={file} showPath={showPath} whiteBox={whiteBox} viewType={viewType} />
                    ))}
                    {showHeadings && filesDeletedNoPrune.length > 0 && (
                        <div className={whiteBox + ' application-resources-diff__heading'}>
                            <b>Removed Resources (pruning disabled)</b>
                            <p>
                                These resources have the <code>argocd.argoproj.io/sync-options: Prune=false</code> annotation, so they will not be pruned.
                                <br />
                                To ignore these resources in the diff, also set the <code>argocd.argoproj.io/compare-options: IgnoreExtraneous</code> annotation.
                            </p>
                        </div>
                    )}
                    {filesDeletedNoPrune.map((file: any) => (
                        <IndividualDiffSection key={file.oldPath} resourceName={file.oldPath} file={file} showPath={showPath} whiteBox={whiteBox} viewType={viewType} />
                    ))}
                </div>
            );
        }}
    </DataLoader>
);
