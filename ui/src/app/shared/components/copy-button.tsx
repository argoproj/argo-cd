import {Tooltip} from 'argo-ui';
import * as React from 'react';
import {useEffect, useRef, useState} from 'react';

import './copy-button.scss';

// The async Clipboard API is only available in secure contexts, so fall back to execCommand elsewhere (e.g. Argo CD served over plain HTTP).
const writeToClipboard = async (text: string): Promise<boolean> => {
    try {
        if (navigator.clipboard?.writeText) {
            await navigator.clipboard.writeText(text);
            return true;
        }
    } catch {
        // fall through to the legacy path
    }
    const textarea = document.createElement('textarea');
    textarea.value = text;
    textarea.setAttribute('readonly', '');
    textarea.style.position = 'fixed';
    textarea.style.opacity = '0';
    document.body.appendChild(textarea);
    textarea.select();
    try {
        return document.execCommand('copy');
    } catch {
        return false;
    } finally {
        document.body.removeChild(textarea);
    }
};

// Copies `text` verbatim, regardless of how the surrounding UI renders it (e.g. upper-cased).
export const CopyButton = ({text, label = 'Copy to clipboard'}: {text: string; label?: string}) => {
    const [status, setStatus] = useState<'idle' | 'copied' | 'failed'>('idle');
    const resetTimeoutRef = useRef<ReturnType<typeof setTimeout>>(undefined);

    const mountedRef = useRef(true);

    useEffect(() => {
        mountedRef.current = true;
        return () => {
            mountedRef.current = false;
            clearTimeout(resetTimeoutRef.current);
        };
    }, []);

    if (!text) {
        return null;
    }

    const copy = async (e: React.SyntheticEvent) => {
        e.stopPropagation();
        const succeeded = await writeToClipboard(text);
        if (!mountedRef.current) {
            return;
        }
        setStatus(succeeded ? 'copied' : 'failed');
        clearTimeout(resetTimeoutRef.current);
        resetTimeoutRef.current = setTimeout(() => setStatus('idle'), 3000);
    };

    const statusText = status === 'copied' ? 'Copied!' : status === 'failed' ? 'Copy failed' : '';

    return (
        <>
            <Tooltip content={statusText || label} hideOnClick={false}>
                <button type='button' className='copy-button' aria-label={label} onClick={copy}>
                    <svg viewBox='0 0 16 16' width='14' height='14' fill='none' stroke='currentColor' strokeWidth='1.5' aria-hidden='true'>
                        <rect x='5.5' y='5.5' width='8' height='8' rx='1' />
                        <path d='M10.5 3.5v-1a1 1 0 0 0-1-1h-7a1 1 0 0 0-1 1v7a1 1 0 0 0 1 1h1' />
                    </svg>
                </button>
            </Tooltip>
            <span role='status' className='copy-button__status'>
                {statusText}
            </span>
        </>
    );
};
