import {NOTIFICATION_SUBSCRIPTION_ANNOTATION_REGEX} from "./edit-notification-subscriptions";

test('rejects incorrect annotations', () => {
    expect(NOTIFICATION_SUBSCRIPTION_ANNOTATION_REGEX.test('notifications_argoproj_io/subscribe_a_b')).toEqual(false)
    expect(NOTIFICATION_SUBSCRIPTION_ANNOTATION_REGEX.test('notifications.argoproj.io/subscribe.a.b')).toEqual(true)
})
