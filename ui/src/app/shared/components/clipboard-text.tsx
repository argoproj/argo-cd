import {Tooltip} from 'argo-ui';
import * as React from 'react';
import {useState} from 'react';

export const ClipboardText = ({text}: {text: string}) => {
    const [justClicked, setJustClicked] = useState<boolean>(false);

    if (!text) {
        return <></>;
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
                            setInterval(() => setJustClicked(false), 3000);
                        }}
                    />
                </a>
            </Tooltip>
        </>
    );
};
