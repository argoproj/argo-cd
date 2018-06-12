import * as React from 'react';

export const ErrorNotification = (props: { title?: string; e: any }) => {
    let message;
    if (props.e.response && props.e.response.text) {
        try {
            const apiError = JSON.parse(props.e.response.text);
            if (apiError.error) {
                message = apiError.error;
            }
        } catch {
            // do nothing
        }
    }
    if (!message) {
        if (props.e.message) {
            message = props.e.message;
        }
    }
    if (!message) {
        message = 'Internal error';
    }
    if (props.title) {
        message = `${props.title}: ${message}`;
    }
    return (<span>{message}</span>);
};
