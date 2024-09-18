import * as React from 'react';
import {useContext} from 'react';
import {Button} from '../../../shared/components/button';
import {Context} from '../../../shared/context';
import {NotificationType} from 'argo-ui/src/components/notifications/notifications';
import {LogEntry} from '../../../shared/models';

// CopyLogsButton is a button that copies the logs to the clipboard
export const CopyLogsButton = ({logs}: {logs: LogEntry[]}) => {
    const ctx = useContext(Context);
    return (
        <Button
            title='Copy logs to clipboard'
            icon='copy'
            onClick={async () => {
                try {
                    await navigator.clipboard.writeText(logs.map(item => item.content).join('\n'));
                    ctx.notifications.show({type: NotificationType.Success, content: 'Copied'}, 750);
                } catch (err) {
                    ctx.notifications.show({type: NotificationType.Error, content: err.message});
                }
            }}
        />
    );
};
