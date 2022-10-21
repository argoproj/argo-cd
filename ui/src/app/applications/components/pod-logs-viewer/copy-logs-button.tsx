import * as React from "react";
import {Tooltip} from "argo-ui";
import {useState} from "react";
import {LogLoader} from "./log-loader";

export const CopyLogsButton = ({
                                   loader,
                               }: { loader: LogLoader}) => {

    const [copy, setCopy] = useState('');
    const setColor = (i: string) => {
        const element = document.getElementById('copyButton');
        if (i === 'success') {
            element.classList.remove('copyStandard');
            element.classList.add('copySuccess');
        } else if (i === 'failure') {
            element.classList.remove('copyStandard');
            element.classList.add('copyFailure');
        } else {
            element.classList.remove('copySuccess');
            element.classList.remove('copyFailure');
            element.classList.add('copyStandard');
        }
    };
    return <Tooltip content='Copy logs'>
        <button
            className='argo-button argo-button--base'
            id='copyButton'
            onClick={async () => {
                try {
                    await navigator.clipboard.writeText(
                        loader
                            .getData()
                            .map(item => item.content)
                            .join('\n')
                    );
                    setCopy('success');
                    setColor('success');
                } catch (err) {
                    setCopy('failure');
                    setColor('failure');
                }
                setTimeout(() => {
                    setCopy('');
                    setColor('');
                }, 750);
            }}>
            {copy === 'success' && (
                <React.Fragment>
                    <i className='fa fa-check'/>
                </React.Fragment>
            )}
            {copy === 'failure' && (
                <React.Fragment>
                    <i className='fa fa-times'/>
                </React.Fragment>
            )}
            {copy === '' && (
                <React.Fragment>
                    <i className='fa fa-copy'/>
                </React.Fragment>
            )}
        </button>
    </Tooltip>
}