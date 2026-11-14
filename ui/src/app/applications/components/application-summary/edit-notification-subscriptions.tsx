import {Autocomplete} from 'argo-ui';
import {useQuery, UseQueryResult} from '@tanstack/react-query';
import * as React from 'react';
import * as models from '../../../shared/models';
import {notificationQueries} from '../../../shared/queries';
import {loading, loadFailed} from '../../../shared/query-client';

import {ApplicationSummaryProps} from './application-summary';

import './edit-notification-subscriptions.scss';

export const NOTIFICATION_SUBSCRIPTION_ANNOTATION_PREFIX = 'notifications.argoproj.io/subscribe';

export const NOTIFICATION_SUBSCRIPTION_ANNOTATION_REGEX = new RegExp(`^notifications\\.argoproj\\.io/subscribe\\.[a-zA-Z-]{1,100}\\.[a-zA-Z-]{1,100}$`);

export type TNotificationSubscription = {
    trigger: string;
    // notification service name
    service: string;
    // a semicolon separated list of recipients
    value: string;
};

export const notificationSubscriptionsParser = {
    annotationsToSubscriptions: (annotations: models.Application['metadata']['annotations']): TNotificationSubscription[] => {
        const subscriptions: TNotificationSubscription[] = [];

        for (const [key, value] of Object.entries(annotations || {})) {
            if (NOTIFICATION_SUBSCRIPTION_ANNOTATION_REGEX.test(key)) {
                try {
                    const [trigger, service] = key.slice(NOTIFICATION_SUBSCRIPTION_ANNOTATION_PREFIX.length + 1 /* for dot "." */).split('.');

                    subscriptions.push({trigger, service, value});
                } catch (e) {
                    // console.error(`annotationsToSubscriptions parsing issue for ${key}`);
                    throw new Error(e);
                }
            }
        }

        return subscriptions;
    },
    subscriptionsToAnnotations: (subscriptions: TNotificationSubscription[]): models.Application['metadata']['annotations'] => {
        const annotations: models.Application['metadata']['annotations'] = {};

        for (const subscription of subscriptions || []) {
            annotations[notificationSubscriptionsParser.subscriptionToAnnotationKey(subscription)] = subscription.value;
        }

        return annotations;
    },
    subscriptionToAnnotationKey: (subscription: TNotificationSubscription): string =>
        `${NOTIFICATION_SUBSCRIPTION_ANNOTATION_PREFIX}.${subscription.trigger}.${subscription.service}`
};

/**
 * split the notification subscription related annotation to have it in separate edit field
 * this hook will emit notification subscription state, controller & merge utility to core annotations helpful when final submit
 */
export const useEditNotificationSubscriptions = (annotations: models.Application['metadata']['annotations']) => {
    const [subscriptions, setSubscriptions] = React.useState(notificationSubscriptionsParser.annotationsToSubscriptions(annotations));

    const onAddNewSubscription = () => {
        const lastSubscription = subscriptions[subscriptions.length - 1];

        if (subscriptions.length === 0 || lastSubscription.trigger || lastSubscription.service || lastSubscription.value) {
            setSubscriptions([
                ...subscriptions,
                {
                    trigger: '',
                    service: '',
                    value: ''
                }
            ]);
        }
    };

    const onEditSubscription = (idx: number, subscription: TNotificationSubscription) => {
        const existingSubscription = subscriptions.findIndex((sub, toFindIdx) => toFindIdx !== idx && sub.service === subscription.service && sub.trigger === subscription.trigger);
        let newSubscriptions = [...subscriptions];

        if (existingSubscription !== -1) {
            // remove existing subscription
            newSubscriptions = newSubscriptions.filter((_, newSubscriptionIdx) => newSubscriptionIdx !== existingSubscription);
            // decrement index because one value is removed
            idx--;
        }

        if (idx === -1) {
            newSubscriptions = [subscription];
        } else {
            newSubscriptions = newSubscriptions.map((oldSubscription, oldSubscriptionIdx) => (oldSubscriptionIdx === idx ? subscription : oldSubscription));
        }

        setSubscriptions(newSubscriptions);
    };

    const onRemoveSubscription = (idx: number) => idx >= 0 && setSubscriptions(subscriptions.filter((_, i) => i !== idx));

    const withNotificationSubscriptions =
        (updateApp: ApplicationSummaryProps['updateApp']) =>
        (...args: Parameters<ApplicationSummaryProps['updateApp']>) => {
            const app = args[0];

            const notificationSubscriptionsRaw = notificationSubscriptionsParser.subscriptionsToAnnotations(subscriptions);

            if (Object.keys(notificationSubscriptionsRaw)?.length) {
                app.metadata.annotations = {
                    ...notificationSubscriptionsRaw,
                    ...(app.metadata.annotations || {})
                };
            }

            return updateApp(app, args[1]);
        };

    const onResetNotificationSubscriptions = () => setSubscriptions(notificationSubscriptionsParser.annotationsToSubscriptions(annotations));

    return {
        /**
         * abstraction of notification subscription annotations in edit view
         */
        subscriptions,
        onAddNewSubscription,
        onEditSubscription,
        onRemoveSubscription,
        /**
         * merge abstracted 'subscriptions' into core 'metadata.annotations' in form submit
         */
        withNotificationSubscriptions,
        onResetNotificationSubscriptions
    };
};

export interface EditNotificationSubscriptionsProps extends ReturnType<typeof useEditNotificationSubscriptions> {}

const SuggestionField = ({
    query,
    title,
    placeholder,
    value,
    onChange
}: {
    query: UseQueryResult<models.NotificationChunk[]>;
    title: 'Trigger' | 'Service';
    placeholder: string;
    value: string;
    onChange: (value: string) => void;
}) => {
    if (!query.data) {
        return query.isError && !query.isFetching ? loadFailed(query.refetch) : loading;
    }
    return (
        <Autocomplete
            wrapperProps={{className: 'argo-field edit-notification-subscriptions__autocomplete-wrapper'}}
            inputProps={{className: 'argo-field', placeholder, title}}
            value={value}
            onChange={e => onChange(e.target.value)}
            items={query.data.map(item => item.name)}
            onSelect={onChange}
            filterSuggestions={true}
            qeid={`application-edit-notification-subscription-${title.toLowerCase()}`}
        />
    );
};

export const EditNotificationSubscriptions = ({subscriptions, onAddNewSubscription, onEditSubscription, onRemoveSubscription}: EditNotificationSubscriptionsProps) => {
    const triggers = useQuery({...notificationQueries.triggers(), enabled: subscriptions.length > 0});
    const notificationServices = useQuery({...notificationQueries.services(), enabled: subscriptions.length > 0});
    const triggersRefreshFailed = triggers.isRefetchError && !triggers.isFetching;
    const servicesRefreshFailed = notificationServices.isRefetchError && !notificationServices.isFetching;
    return (
        <div className='edit-notification-subscriptions argo-field'>
            {subscriptions.length > 0 &&
                (triggersRefreshFailed || servicesRefreshFailed) &&
                loadFailed(() => {
                    if (triggersRefreshFailed) {
                        triggers.refetch();
                    }
                    if (servicesRefreshFailed) {
                        notificationServices.refetch();
                    }
                })}
            {subscriptions.map((subscription, idx) => (
                <div className='edit-notification-subscriptions__subscription' key={idx}>
                    <input className='argo-field edit-notification-subscriptions__input-prefix' disabled={true} value={NOTIFICATION_SUBSCRIPTION_ANNOTATION_PREFIX} />
                    <b>&nbsp;.&nbsp;</b>
                    <SuggestionField
                        query={triggers}
                        title='Trigger'
                        placeholder='on-sync-running'
                        value={subscription.trigger}
                        onChange={trigger => onEditSubscription(idx, {...subscription, trigger})}
                    />
                    <b>&nbsp;.&nbsp;</b>
                    <SuggestionField
                        query={notificationServices}
                        title='Service'
                        placeholder='slack'
                        value={subscription.service}
                        onChange={service => onEditSubscription(idx, {...subscription, service})}
                    />
                    &nbsp;=&nbsp;
                    <input
                        autoComplete='fake'
                        className='argo-field'
                        placeholder='my-channel1; my-channel2'
                        title='Value'
                        value={subscription.value}
                        onChange={e => {
                            onEditSubscription(idx, {
                                ...subscription,
                                value: e.target.value
                            });
                        }}
                        qe-id='application-edit-notification-subscription-value'
                    />
                    <button className='button-close'>
                        <i className='fa fa-times' style={{cursor: 'pointer'}} onClick={() => onRemoveSubscription(idx)} />
                    </button>
                </div>
            ))}
            {subscriptions.length === 0 && <label>No items</label>}
            <div>
                <button className='argo-button argo-button--base argo-button--short' onClick={() => onAddNewSubscription()}>
                    <i className='fa fa-plus' style={{cursor: 'pointer'}} />
                </button>
            </div>
        </div>
    );
};
