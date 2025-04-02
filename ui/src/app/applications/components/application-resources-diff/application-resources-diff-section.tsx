import * as React from 'react';
import {addQueueItem, clearQueue} from './diff-queue';
import {parseDiff} from 'react-diff-view';
import {useVirtualizer} from '@tanstack/react-virtual';
import {IndividualDiffSection} from './individual-diff-section';

interface DiffFileModel {
    loading: boolean;
    file?: any;
}

export interface ApplicationResourcesDiffSectionProps {
    prepareDiff: {
        a: string;
        b: string;
        name: string;
        hook?: boolean;
    }[];
    compactDiff: boolean;
    inlineDiff: boolean;
}

export const ApplicationResourcesDiffSection = (props: ApplicationResourcesDiffSectionProps) => {
    const {prepareDiff, compactDiff, inlineDiff} = props;
    const parentRef = React.useRef();

    const whiteBox = prepareDiff.length > 1 ? 'white-box' : '';
    const showPath = prepareDiff.length > 1;
    const viewType = inlineDiff ? 'unified' : 'split';

    const [diffProgress, setDiffProgress] = React.useState({
        finished: 0,
        total: prepareDiff.length
    });

    const [diffFiles, setDiffFiles] = React.useState<DiffFileModel[]>(
        prepareDiff.map(() => ({
            loading: true,
            file: {}
        }))
    );

    React.useEffect(() => {
        prepareDiff.forEach((filePrepare, i) => {
            addQueueItem({
                a: filePrepare.a,
                b: filePrepare.b,
                name: filePrepare.name,
                compactDiff: compactDiff,
                inlineDiff: inlineDiff
            }).then((diffText: string) => {
                const files = parseDiff(diffText);

                setDiffFiles(diffLines => {
                    const newDiffFiles = [...diffLines];
                    newDiffFiles[i] = {
                        loading: false,
                        file: files[0]
                    };

                    setDiffProgress(val => {
                        return {
                            ...val,
                            finished: val.finished + 1
                        };
                    });

                    return newDiffFiles;
                });
            });
        });

        return () => {
            setDiffProgress({
                finished: 0,
                total: prepareDiff.length
            });
            setDiffFiles(
                prepareDiff.map(() => ({
                    loading: true,
                    file: {}
                }))
            );
            clearQueue();
        };
    }, [compactDiff, prepareDiff]);

    // The virtualizer
    const rowVirtualizer = useVirtualizer({
        count: prepareDiff.length,
        getScrollElement: () => parentRef.current,
        estimateSize: () => 35
    });

    const virtualizeItems = rowVirtualizer.getVirtualItems();

    return (
        <>
            {diffProgress.finished < diffProgress.total && (
                <div style={{marginBottom: '10px'}}>
                    {diffProgress.finished} / {diffProgress.total} files diff
                </div>
            )}
            <div
                ref={parentRef}
                style={{
                    height: 'calc(100vh - 260px)',
                    width: '100%',
                    overflowY: 'auto',
                    contain: 'strict'
                }}>
                <div
                    style={{
                        height: rowVirtualizer.getTotalSize(),
                        width: '100%',
                        position: 'relative'
                    }}>
                    <div
                        style={{
                            position: 'absolute',
                            top: 0,
                            left: 0,
                            width: '100%',
                            transform: `translateY(${virtualizeItems[0]?.start ?? 0}px)`
                        }}>
                        {virtualizeItems.map((virtualRow: any) => (
                            <IndividualDiffSection
                                key={virtualRow.key}
                                dataIndex={virtualRow.index}
                                ref={rowVirtualizer.measureElement}
                                file={diffFiles[virtualRow.index].file}
                                showPath={showPath}
                                loading={diffFiles[virtualRow.index].loading}
                                whiteBox={whiteBox}
                                viewType={viewType}
                            />
                        ))}
                    </div>
                </div>
            </div>
        </>
    );
};
