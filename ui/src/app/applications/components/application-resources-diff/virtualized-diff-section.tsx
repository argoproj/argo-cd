import * as React from 'react';
import {useState, useMemo, useCallback} from 'react';
import {Diff, Hunk, tokenize, markEdits} from 'react-diff-view';
import 'react-diff-view/style/index.css';
import {YamlPerformanceInfo, analyzeYamlPerformance} from '../../../shared/utils/yaml-performance';

import './application-resources-diff.scss';

export interface VirtualizedDiffSectionProps {
    file: any;
    showPath: boolean;
    whiteBox: string;
    viewType: string;
}

// Virtualization constants
const LINES_PER_CHUNK = 100;
const INITIAL_CHUNKS = 2;

export const VirtualizedDiffSection = (props: VirtualizedDiffSectionProps) => {
    const {file, showPath, whiteBox, viewType} = props;
    const [collapsed, setCollapsed] = useState(false);
    const [visibleChunks, setVisibleChunks] = useState(INITIAL_CHUNKS);
    
    // Analyze performance characteristics
    const performanceInfo = useMemo(() => {
        const fullContent = file.hunks.map((hunk: any) => hunk.content).join('\n');
        return analyzeYamlPerformance(fullContent);
    }, [file.hunks]);
    
    // Virtualize hunks for large files
    const {virtualizedHunks, hasMore} = useMemo(() => {
        if (!performanceInfo.isLarge) {
            return { virtualizedHunks: file.hunks, hasMore: false };
        }
        
        const totalLines = file.hunks.reduce((acc: number, hunk: any) => acc + hunk.changes.length, 0);
        const maxVisibleLines = visibleChunks * LINES_PER_CHUNK;
        
        let currentLines = 0;
        const visibleHunks: any[] = [];
        
        for (const hunk of file.hunks) {
            if (currentLines + hunk.changes.length <= maxVisibleLines) {
                visibleHunks.push(hunk);
                currentLines += hunk.changes.length;
            } else {
                // Add partial hunk if we have space
                const remainingLines = maxVisibleLines - currentLines;
                if (remainingLines > 0) {
                    const partialHunk = {
                        ...hunk,
                        changes: hunk.changes.slice(0, remainingLines)
                    };
                    visibleHunks.push(partialHunk);
                }
                break;
            }
        }
        
        return {
            virtualizedHunks: visibleHunks,
            hasMore: currentLines < totalLines
        };
    }, [file.hunks, visibleChunks, performanceInfo.isLarge]);
    
    const loadMore = useCallback(() => {
        setVisibleChunks(prev => prev + 1);
    }, []);
    
    const options = {
        highlight: false,
        enhancers: [markEdits(virtualizedHunks, {type: 'block'})]
    };
    const token = tokenize(virtualizedHunks, options);
    
    return (
        <div className={`${whiteBox} application-component-diff__diff`}>
            {showPath && (
                <p className='application-resources-diff__diff__title'>
                    {file.newPath}
                    <i className={`fa fa-caret-${collapsed ? 'down' : 'up'} diff__collapse`} onClick={() => setCollapsed(!collapsed)} />
                </p>
            )}
            
            {performanceInfo.warningMessage && (
                <div className='application-resources-diff__warning'>
                    <i className='fa fa-exclamation-triangle' />
                    <span>{performanceInfo.warningMessage}</span>
                </div>
            )}
            
            {!collapsed && (
                <div>
                    <Diff viewType={viewType} diffType={file.type} hunks={virtualizedHunks} tokens={token}>
                        {(hunks: any) => hunks.map((hunk: any) => <Hunk className={'custom-diff-hunk'} key={hunk.content} hunk={hunk} />)}
                    </Diff>
                    
                    {hasMore && (
                        <div className='application-resources-diff__load-more'>
                            <button 
                                className='argo-button argo-button--base'
                                onClick={loadMore}
                            >
                                Load More ({Math.max(0, file.hunks.reduce((acc: number, hunk: any) => acc + hunk.changes.length, 0) - visibleChunks * LINES_PER_CHUNK)} lines remaining)
                            </button>
                        </div>
                    )}
                </div>
            )}
        </div>
    );
};
