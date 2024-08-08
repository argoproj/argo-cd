import {Tooltip} from 'argo-ui';
import * as React from 'react';
import {useEffect, useState} from 'react';

export const ClipboardText = ({text}: {text: string}) => {
    const [isCopied, setIsCopied] = useState<boolean>(false);

    if (!text) {
        return <></>;
    }

    const handleCopyClick = () => {
        setIsCopied(true);
        navigator.clipboard.writeText(text);
    };

    useEffect(() => {
        if (isCopied) {
            const timer = setTimeout(() => setIsCopied(false), 2000);
            return () => clearTimeout(timer);
        }
    }, [isCopied]);

    return (
        <>
            {text}
            &nbsp; &nbsp;
            <Tooltip content={isCopied ? 'Copied!' : 'Copy to clipboard'}>
                <a>
                    <i className='fa fa-clipboard' onClick={handleCopyClick} />
                </a>
            </Tooltip>
        </>
    );
};
