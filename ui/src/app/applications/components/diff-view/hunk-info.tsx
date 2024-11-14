import * as React from 'react';
import {Decoration, DecorationProps, HunkData} from 'react-diff-view';

interface Props extends Omit<DecorationProps, 'children'> {
    hunk: HunkData;
}

export default function HunkInfo({hunk, ...props}: Props) {
    return (
        <Decoration {...props}>
            {null}
            {hunk.content}
        </Decoration>
    );
}
