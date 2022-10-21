import * as React from 'react';
import {useContext} from 'react';
import {LogLoader} from './log-loader';
import {Button} from '../../../shared/components/button';
import {Context} from '../../../shared/context';
import {NotificationType} from 'argo-ui/src/components/notifications/notifications';

export const CopyLogsButton = ({loader}: {loader: LogLoader}) => {
    const ctx = useContext(Context);
    return (
        <Button
            title='Copy logs'
            icon='copy'
            onClick={async () => {
                try {
                    await navigator.clipboard.writeText(
                        loader
                            .getData()
                            .map(item => item.content)
                            .join('\n')
                    );
                    ctx.notifications.show({type: NotificationType.Success, content: 'Copied'}, 750);
                } catch (err) {
                    ctx.notifications.show({type: NotificationType.Error, content: err.message});
                }
            }}
        />
    );
};
