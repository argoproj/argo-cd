import {MouseEvent, useCallback, useMemo, useState} from 'react';
import {ChangeEventArgs, getChangeKey, HunkData} from 'react-diff-view';

interface Selection {
    start: string | null;
    end: string | null;
}

interface InlineState {
    inSelection: boolean;
    keys: string[];
}

export const useSelection = (hunks: HunkData[]) => {
    const [{start, end}, setSelection] = useState<Selection>({start: null, end: null});
    const [currentHunks, setCurrentHunks] = useState(hunks);
    const select = useCallback(
        ({change}: ChangeEventArgs, e: MouseEvent<HTMLElement>) => {
            if (!change) {
                return;
            }

            const key = getChangeKey(change);
            if (e.shiftKey && start) {
                setSelection(v => ({start: v.start, end: key}));
            } else {
                setSelection({start: key, end: key});
            }
        },
        [start]
    );
    const selected = useMemo(() => {
        if (!start || !end) {
            return [];
        }

        if (start === end) {
            return [start];
        }

        // Find all changes from start to end in all hunks
        const state: InlineState = {
            inSelection: false,
            keys: []
        };
        for (const hunk of currentHunks) {
            for (const change of hunk.changes) {
                const key = getChangeKey(change);
                if (key === start || key === end) {
                    state.keys.push(key);
                    state.inSelection = !state.inSelection;
                } else if (state.inSelection) {
                    state.keys.push(key);
                }
            }
        }
        return state.keys;
    }, [currentHunks, end, start]);

    if (hunks !== currentHunks) {
        setSelection({start: null, end: null});
        setCurrentHunks(hunks);
    }

    return [selected, select] as const;
};
