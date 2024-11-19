import * as React from 'react';
import {ReactElement, useMemo} from 'react';
import {Diff, Hunk, useSourceExpansion, useMinCollapsedLines, HunkData, MarkEditsType, DiffType, EventMap, ViewType} from 'react-diff-view';
import 'react-diff-view/style/index.css';

import HunkInfo from './hunk-info';
import UnfoldCollapsed from './unfold-collapsed';
import {useSelection} from './selection';
import tokenize from './tokenize';
import './diff-view.scss';

interface EnhanceOptions {
    editsType: MarkEditsType;
    language: string;
}

function useEnhance(hunks: HunkData[], oldSource: string | null, {editsType, language}: EnhanceOptions) {
    const [hunksWithSourceExpanded, expandRange] = useSourceExpansion(hunks, oldSource);
    const hunksWithMinLinesCollapsed = useMinCollapsedLines(0, hunksWithSourceExpanded, oldSource);
    const [selection, toggleSelection] = useSelection(hunksWithMinLinesCollapsed);
    const tokens = useMemo(() => {
        return tokenize(hunksWithMinLinesCollapsed, editsType, oldSource, language);
    }, [hunksWithMinLinesCollapsed, editsType, oldSource, language]);
    return {
        expandRange,
        selection,
        toggleSelection,
        tokens,
        hunks: hunksWithMinLinesCollapsed
    };
}

interface Props {
    diffType: DiffType;
    viewType: ViewType;
    editsType: MarkEditsType;
    language: string;
    hunks: HunkData[];
    oldSource: string | null;
}

export default function DiffView(props: Props) {
    const {oldSource, diffType, viewType, editsType, language} = props;
    const {expandRange, selection, toggleSelection, tokens, hunks} = useEnhance(props.hunks, oldSource, {editsType, language});
    const events: EventMap = {
        onClick: toggleSelection
    };
    const linesCount = oldSource ? oldSource.split('\n').length : 0;
    const renderHunk = (children: ReactElement[], hunk: HunkData, i: number, hunks: HunkData[]) => {
        const previousElement = children[children.length - 1];
        const decorationElement = oldSource ? (
            <UnfoldCollapsed
                key={`decoration-${hunk.content}`}
                previousHunk={previousElement && previousElement.props.hunk}
                currentHunk={hunk}
                linesCount={linesCount}
                onExpand={expandRange}
            />
        ) : (
            <HunkInfo key={`decoration-${hunk.content}`} hunk={hunk} />
        );
        children.push(decorationElement);

        const hunkElement = <Hunk key={`hunk-${hunk.content}`} hunk={hunk} />;
        children.push(hunkElement);

        if (i === hunks.length - 1 && oldSource) {
            const unfoldTailElement = <UnfoldCollapsed key='decoration-tail' previousHunk={hunk} linesCount={linesCount} onExpand={expandRange} />;
            children.push(unfoldTailElement);
        }

        return children;
    };

    return (
        <Diff optimizeSelection viewType={viewType} diffType={diffType} hunks={hunks} selectedChanges={selection} tokens={tokens} codeEvents={events} gutterEvents={events}>
            {hunks => hunks.reduce(renderHunk, [])}
        </Diff>
    );
}
