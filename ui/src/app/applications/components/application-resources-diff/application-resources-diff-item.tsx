import React from 'react';
import {diffLines, formatLines} from 'unidiff';
import {parseDiff} from 'react-diff-view';
import {IndividualDiffSection} from './individual-diff-section';

const calcMaxTime = 100;

interface DiffParams {
    compactDiff: boolean;
    inlineDiff: boolean;
    a: string;
    b: string;
    name: string;
    hook: boolean;
}

const diffLineFn = (params: DiffParams) => {
    const context = params.compactDiff ? 2 : Number.MAX_SAFE_INTEGER;
    const diffLineResult = `diff --git a/${params.name} b/${params.name}
index 6829b8a2..4c565f1b 100644
${formatLines(diffLines(params.a, params.b), {context, aname: `a/${name}}`, bname: `b/${params.name}`})}`;
    // react-diff-view, awesome as it is, does not accept unidiff format, you must add a git header section
    return diffLineResult;
};

export interface AsyncDiffModel {
    handler: Promise<any>;
    params: any;
    resolve?: CallableFunction;
    reject?: CallableFunction;
}

interface DiffFileModel {
    [key: string]: any;
}

export const diffQueue: AsyncDiffModel[] = [];

let startIdx = 0;

export function addQueueItem<T>(item: T) {
    let resolveHandler;
    let rejectHandler;

    const handler = new Promise((resolve, reject) => {
        resolveHandler = resolve;
        rejectHandler = reject;
    });

    const newItem: AsyncDiffModel = {
        params: item,
        handler,
        resolve: resolveHandler,
        reject: rejectHandler
    };

    diffQueue.push(newItem);

    return handler;
}

export function clearQueue() {
    diffQueue.length = 0;
    startIdx = 0;
}

let enabled = false;

export function enableQueue() {
    enabled = true;
    processQueue();
}

export function disableQueue() {
    enabled = false;
}

export function processQueue() {
    const startTime = performance.now();

    for (let i = startIdx; i < diffQueue.length; i++) {
        try {
            const rst = diffLineFn(diffQueue[i].params);
            diffQueue[i].resolve(rst);
        } catch {
            diffQueue[i].reject();
        }

        if (performance.now() - startTime > calcMaxTime && i < diffQueue.length) {
            startIdx = i + 1;
            break;
        }
    }

    if (enabled) {
        requestAnimationFrame(processQueue);
    }
}

export interface ApplicationResourcesDiffItemProps {
    compactDiff: boolean;
    inlineDiff: boolean;
    a: string;
    b: string;
    name: string;
    whiteBox: string;
    showPath: boolean;
    hook: boolean;
}

export const ApplicationResourcesDiffItem = (props: ApplicationResourcesDiffItemProps) => {
    const {showPath, whiteBox, inlineDiff} = props;
    const viewType = inlineDiff ? 'unified' : 'split';
    const [loading, updateLoading] = React.useState(true);
    const [diffFile, setDiffFile] = React.useState<DiffFileModel>({});

    let isExit = false;

    React.useEffect(() => {
        updateLoading(true);
        addQueueItem({
            compactDiff: props.compactDiff,
            a: props.a,
            b: props.b,
            name: props.name
        }).then((diffText: string) => {
            const files = parseDiff(diffText);
            if (isExit) {
                return;
            }
            setDiffFile({
                ...files[0]
            });

            updateLoading(false);
        });

        return () => {
            isExit = true;
        };
    }, [showPath, inlineDiff, props.compactDiff, props.a, props.b, props.name]);

    return <IndividualDiffSection file={diffFile} showPath={showPath} loading={loading} whiteBox={whiteBox} viewType={viewType} />;
};
