import * as React from 'react';
import {DataLoader} from '../../../shared/components';
import {services, ViewPreferences} from '../../../shared/services';
import {addDismissal, dismissalKey, ParsedNotice, parseNotice, shouldShowBanner} from './notice';
import './notice.scss';

interface NoticeBannerProps {
    annotations: {[key: string]: string} | undefined;
    appName: string;
    appNamespace: string;
}

export const NoticeBanner = ({annotations, appName, appNamespace}: NoticeBannerProps) => {
    const notice = parseNotice(annotations);
    if (!notice || !shouldShowBanner(notice)) {
        return null;
    }
    return (
        <DataLoader load={() => services.viewPreferences.getPreferences()}>
            {(prefs: ViewPreferences) => <NoticeBannerContent notice={notice} prefs={prefs} appName={appName} appNamespace={appNamespace} />}
        </DataLoader>
    );
};

const NoticeBannerContent = ({notice, prefs, appName, appNamespace}: {notice: ParsedNotice; prefs: ViewPreferences; appName: string; appNamespace: string}) => {
    const dismissed = prefs.dismissedNotices || {};
    const key = dismissalKey(appNamespace, appName, notice.content);
    if (!notice.permanent && dismissed[key]) {
        return null;
    }

    const dismiss = () => {
        services.viewPreferences.updatePreferences({dismissedNotices: addDismissal(dismissed, key)});
    };

    const content = notice.url ? (
        <a href={notice.url} target='_blank' rel='noopener noreferrer'>
            {notice.content}
        </a>
    ) : (
        <>{notice.content}</>
    );

    return (
        <div className={`application-notice application-notice--${notice.severity}`} role={notice.severity === 'critical' ? 'alert' : 'status'}>
            <i className={`application-notice__icon fa ${notice.iconClass}`} aria-hidden='true' />
            <div className='application-notice__content'>{content}</div>
            {!notice.permanent && (
                <button type='button' className='application-notice__dismiss' onClick={dismiss} title='Dismiss'>
                    <i className='fa fa-times' aria-hidden='true' />
                </button>
            )}
        </div>
    );
};
