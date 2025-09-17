import {Checkbox, DataLoader} from 'argo-ui';
import * as jsYaml from 'js-yaml';
import * as React from 'react';
import {parseDiff} from 'react-diff-view';
import 'react-diff-view/style/index.css';
import {diffLines, formatLines} from 'unidiff';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {IndividualDiffSection} from './individual-diff-section';
import {VirtualizedDiffSection} from './virtualized-diff-section';
import {safeYamlDump, optimizeDiffContext, truncateDiffContent, YAML_SIZE_LIMITS} from '../../../shared/utils/yaml-performance';

import './application-resources-diff.scss';

export interface ApplicationResourcesDiffProps {
    states: models.ResourceDiff[];
}

export const ApplicationResourcesDiff = (props: ApplicationResourcesDiffProps) => (
    <DataLoader key='resource-diff' load={() => services.viewPreferences.getPreferences()}>
        {pref => {
            const processedStates = props.states
                .map(state => {
                    const { yaml: a, info: infoA } = state.normalizedLiveState ? safeYamlDump(state.normalizedLiveState) : { yaml: '', info: null };
                    const { yaml: b, info: infoB } = state.predictedLiveState ? safeYamlDump(state.predictedLiveState) : { yaml: '', info: null };
                    
                    return {
                        a,
                        b,
                        hook: state.hook,
                        // doubles as sort order
                        name: (state.group || '') + '/' + state.kind + '/' + (state.namespace ? state.namespace + '/' : '') + state.name,
                        performanceInfo: { infoA, infoB }
                    };
                })
                .filter(i => !i.hook)
                .filter(i => i.a !== i.b);

            const diffText = processedStates
                .map(i => {
                    const baseContext = pref.appDetails.compactDiff ? 2 : YAML_SIZE_LIMITS.MAX_CONTEXT_LINES;
                    const context = optimizeDiffContext(baseContext, Math.max(i.a.length, i.b.length));
                    
                    // react-diff-view, awesome as it is, does not accept unidiff format, you must add a git header section
                    return `diff --git a/${i.name} b/${i.name}
index 6829b8a2..4c565f1b 100644
${formatLines(diffLines(i.a, i.b), {context, aname: `a/${i.name}`, bname: `b/${i.name}`})}`;
                })
                .join('\n');

            // Truncate diff content if it's too large
            const { content: truncatedDiffText, wasTruncated } = truncateDiffContent(diffText);
            // assume that if you only have one file, we don't need the file path
            const whiteBox = props.states.length > 1 ? 'white-box' : '';
            const showPath = props.states.length > 1;
            const files = parseDiff(truncatedDiffText);
            const viewType = pref.appDetails.inlineDiff ? 'unified' : 'split';
            
            return (
                <div className='application-resources-diff'>
                    {wasTruncated && (
                        <div className='application-resources-diff__global-warning'>
                            <i className='fa fa-exclamation-triangle' />
                            <span>Diff content has been truncated for performance. Some changes may not be visible.</span>
                        </div>
                    )}
                    
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
                        .map((file: any) => {
                            // Use virtualized component for large files
                            const isLargeFile = processedStates.some(state => 
                                state.name === file.newPath && 
                                (state.performanceInfo.infoA?.isLarge || state.performanceInfo.infoB?.isLarge)
                            );
                            
                            if (isLargeFile) {
                                return (
                                    <VirtualizedDiffSection 
                                        key={file.newPath} 
                                        file={file} 
                                        showPath={showPath} 
                                        whiteBox={whiteBox} 
                                        viewType={viewType} 
                                    />
                                );
                            } else {
                                return (
                                    <IndividualDiffSection 
                                        key={file.newPath} 
                                        file={file} 
                                        showPath={showPath} 
                                        whiteBox={whiteBox} 
                                        viewType={viewType} 
                                    />
                                );
                            }
                        })}
                </div>
            );
        }}
    </DataLoader>
);
