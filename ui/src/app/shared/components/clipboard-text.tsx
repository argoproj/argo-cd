import {Tooltip} from 'argo-ui';
import * as React from 'react';
import {useEffect, useRef, useState} from 'react';

export const ClipboardText = ({text}: {text: string}) => {
    const [justClicked, setJustClicked] = useState<boolean>(false);
    const resetTimeoutRef = useRef<ReturnType<typeof setTimeout>>(undefined);

    useEffect(() => () => clearTimeout(resetTimeoutRef.current), []);

    if (!text) {
        return null;
    }

    return (
        <>
            {text}
            &nbsp; &nbsp;
            <Tooltip content={justClicked ? 'Copied!' : 'Copy to clipboard'} hideOnClick={false}>
                <a>
                    <i
                        className={'fa fa-clipboard'}
                        onClick={() => {
                            setJustClicked(true);
                            navigator.clipboard.writeText(text);
                            clearTimeout(resetTimeoutRef.current);
                            resetTimeoutRef.current = setTimeout(() => setJustClicked(false), 3000);
                        }}
                    />
                </a>
            </Tooltip>
        </>
    );
};
