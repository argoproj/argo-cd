import * as React from 'react';
import {getCollapsedLinesCountBetween, HunkData} from 'react-diff-view';
import Unfold from './unfold';

interface Props {
    previousHunk: HunkData;
    currentHunk?: HunkData;
    linesCount: number;
    onExpand: (start: number, end: number) => void;
}

export default function UnfoldCollapsed({previousHunk, currentHunk, linesCount, onExpand}: Props) {
    if (!currentHunk) {
        const nextStart = previousHunk.oldStart + previousHunk.oldLines;
        const collapsedLines = linesCount - nextStart + 1;

        if (collapsedLines <= 0) {
            return null;
        }

        return (
            <>
                {collapsedLines > 10 && <Unfold direction='down' start={nextStart} end={nextStart + 10} onExpand={onExpand} />}
                <Unfold direction='none' start={nextStart} end={linesCount + 1} onExpand={onExpand} />
            </>
        );
    }

    const collapsedLines = getCollapsedLinesCountBetween(previousHunk, currentHunk);

    if (!previousHunk) {
        if (!collapsedLines) {
            return null;
        }

        const start = Math.max(currentHunk.oldStart - 10, 1);

        return (
            <>
                <Unfold direction='none' start={1} end={currentHunk.oldStart} onExpand={onExpand} />
                {collapsedLines > 10 && <Unfold direction='up' start={start} end={currentHunk.oldStart} onExpand={onExpand} />}
            </>
        );
    }

    const collapsedStart = previousHunk.oldStart + previousHunk.oldLines;
    const collapsedEnd = currentHunk.oldStart;

    if (collapsedLines < 10) {
        return <Unfold direction='none' start={collapsedStart} end={collapsedEnd} onExpand={onExpand} />;
    }

    return (
        <>
            <Unfold direction='down' start={collapsedStart} end={collapsedStart + 10} onExpand={onExpand} />
            <Unfold direction='none' start={collapsedStart} end={collapsedEnd} onExpand={onExpand} />
            <Unfold direction='up' start={collapsedEnd - 10} end={collapsedEnd} onExpand={onExpand} />
        </>
    );
}
