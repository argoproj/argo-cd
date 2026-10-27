import * as React from 'react';
import {FormField} from 'argo-ui';
import {FormApi} from 'argo-ui';
import * as models from '../../../shared/models';
import {MapInputField} from '../../../shared/components';
import {notificationSubscriptionsParser} from './edit-notification-subscriptions';

export const EditAnnotations = (props: {formApi: FormApi; app: models.Application}) => {
    React.useEffect(() => {
        const annotations = props.app.metadata.annotations;
        const notificationSubscriptions = notificationSubscriptionsParser.annotationsToSubscriptions(annotations);

        if (notificationSubscriptions.length > 0) {
            const annotationsWithoutNotificationSubscriptions = {...(annotations || {})};

            for (const notificationSubscriptionAnnotation of notificationSubscriptions) {
                const key = notificationSubscriptionsParser.subscriptionToAnnotationKey(notificationSubscriptionAnnotation);

                delete annotationsWithoutNotificationSubscriptions[key];
            }

            props.formApi.setValue('metadata.annotations', annotationsWithoutNotificationSubscriptions);
        }
        // Strip subscription keys once when the editor mounts. Re-running on app/form
        // updates would wipe annotations the user is in the middle of editing.
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    return <FormField formApi={props.formApi} field='metadata.annotations' component={MapInputField} />;
};
