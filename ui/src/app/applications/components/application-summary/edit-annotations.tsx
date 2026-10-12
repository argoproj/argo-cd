import * as React from 'react';
import {FormField} from 'argo-ui';
import {FormApi} from 'argo-ui';
import * as models from '../../../shared/models';
import {MapInputField} from '../../../shared/components';
import {notificationSubscriptionsParser} from './edit-notification-subscriptions';

export const EditAnnotations = (props: {formApi: FormApi; app: models.Application}) => {
    const removeNotificationSubscriptionRelatedAnnotations = () => {
        const notificationSubscriptions = notificationSubscriptionsParser.annotationsToSubscriptions(props.app.metadata.annotations);

        if (notificationSubscriptions.length > 0) {
            const annotationsWithoutNotificationSubscriptions = {...(props.app.metadata.annotations || {})};

            for (const notificationSubscriptionAnnotation of notificationSubscriptions) {
                const key = notificationSubscriptionsParser.subscriptionToAnnotationKey(notificationSubscriptionAnnotation);

                delete annotationsWithoutNotificationSubscriptions[key];
            }

            props.formApi.setValue('metadata.annotations', annotationsWithoutNotificationSubscriptions);
        }
    };

    React.useEffect(() => {
        removeNotificationSubscriptionRelatedAnnotations();
    }, []);

    return <FormField formApi={props.formApi} field='metadata.annotations' component={MapInputField} />;
};
