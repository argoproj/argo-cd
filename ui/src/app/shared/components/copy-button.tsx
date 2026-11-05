import {Tooltip} from 'argo-ui';
import * as React from 'react';
import {useEffect, useRef, useState} from 'react';

import './copy-button.scss';

// Copies `text` verbatim, regardless of how the surrounding UI renders it (e.g. upper-cased).
export const CopyButton = ({text, label = 'Copy to clipboard'}: {text: string; label?: string}) => {
    const [justClicked, setJustClicked] = useState<boolean>(false);
    const resetTimeoutRef = useRef<ReturnType<typeof setTimeout>>(undefined);

    useEffect(() => () => clearTimeout(resetTimeoutRef.current), []);

    if (!text) {
        return null;
    }

    const copy = (e: React.SyntheticEvent) => {
        e.stopPropagation();
        navigator.clipboard.writeText(text);
        setJustClicked(true);
        clearTimeout(resetTimeoutRef.current);
        resetTimeoutRef.current = setTimeout(() => setJustClicked(false), 3000);
    };

    return (
        <Tooltip content={justClicked ? 'Copied!' : label} hideOnClick={false}>
            <button type='button' className='copy-button' aria-label={label} onClick={copy}>
                <svg viewBox='0 0 16 16' width='14' height='14' fill='none' stroke='currentColor' strokeWidth='1.5' aria-hidden='true'>
                    <rect x='5.5' y='5.5' width='8' height='8' rx='1' />
                    <path d='M10.5 3.5v-1a1 1 0 0 0-1-1h-7a1 1 0 0 0-1 1v7a1 1 0 0 0 1 1h1' />
                </svg>
            </button>
        </Tooltip>
    );
};
