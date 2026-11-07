import {Tooltip} from 'argo-ui';
import * as React from 'react';
import {useEffect, useRef, useState} from 'react';

const copyWithTextarea = (text: string): boolean => {
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

const writeToClipboard = (text: string) => {
    if (navigator.clipboard?.writeText) {
        navigator.clipboard.writeText(text).catch(() => copyWithTextarea(text));
        return;
    }
    copyWithTextarea(text);
};

export const ClipboardText = ({text, children, label = 'Copy to clipboard'}: {text: string; children?: React.ReactNode; label?: string}) => {
    const [justClicked, setJustClicked] = useState<boolean>(false);
    const resetTimeoutRef = useRef<ReturnType<typeof setTimeout>>(undefined);

    useEffect(() => () => clearTimeout(resetTimeoutRef.current), []);

    if (!text) {
        return null;
    }

    return (
        <>
            {children !== undefined ? children : text}
            <Tooltip content={justClicked ? 'Copied!' : label} hideOnClick={false}>
                <a
                    style={{marginLeft: '0.5em'}}
                    aria-label={label}
                    onClick={e => {
                        e.stopPropagation();
                        setJustClicked(true);
                        writeToClipboard(text);
                        clearTimeout(resetTimeoutRef.current);
                        resetTimeoutRef.current = setTimeout(() => setJustClicked(false), 3000);
                    }}>
                    <i className={'fa fa-clipboard'} aria-hidden='true' />
                </a>
            </Tooltip>
        </>
    );
};
