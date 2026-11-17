import * as React from 'react';
import {act, fireEvent, render, screen} from '@testing-library/react';
import '@testing-library/jest-dom';

import {services} from '../../../shared/services';
import {dismissalKey, NoticeAnnotationContent, NoticeAnnotationPermanent, NoticeAnnotationURL} from './notice';
import {NoticeBanner} from './notice-banner';

jest.mock('../../../shared/services', () => {
    const {ViewPreferencesService} = jest.requireActual('../../../shared/services/view-preferences-service');
    return {services: {viewPreferences: new ViewPreferencesService()}};
});

const content = 'Payments freeze until Friday';

const renderBanner = (annotations: {[key: string]: string}) => render(<NoticeBanner annotations={annotations} appName='payments' appNamespace='argocd' />);

describe('NoticeBanner', () => {
    beforeEach(() => {
        window.localStorage.clear();
        services.viewPreferences.init();
        // init() keeps the in-memory preferences after the first test, so clear dismissals explicitly.
        services.viewPreferences.updatePreferences({dismissedNotices: {}});
    });

    it('shows the notice with its link right away', () => {
        renderBanner({[NoticeAnnotationContent]: content, [NoticeAnnotationURL]: 'https://wiki.example.com/freeze'});

        expect(screen.getByRole('link', {name: content})).toHaveAttribute('href', 'https://wiki.example.com/freeze');
        expect(screen.queryByText('Loading...')).not.toBeInTheDocument();
    });

    it('hides the notice once dismissed and remembers the dismissal', () => {
        renderBanner({[NoticeAnnotationContent]: content});

        fireEvent.click(screen.getByTitle('Dismiss'));

        expect(screen.queryByText(content)).not.toBeInTheDocument();
        expect(services.viewPreferences.getCurrentPreferences().dismissedNotices).toEqual({[dismissalKey('argocd', 'payments', content)]: true});
    });

    it('does not show a notice that was dismissed before', () => {
        services.viewPreferences.updatePreferences({dismissedNotices: {[dismissalKey('argocd', 'payments', content)]: true}});
        renderBanner({[NoticeAnnotationContent]: content});

        expect(screen.queryByText(content)).not.toBeInTheDocument();
    });

    it('hides the notice when it is dismissed elsewhere, such as in another tab', () => {
        renderBanner({[NoticeAnnotationContent]: content});
        expect(screen.getByText(content)).toBeInTheDocument();

        act(() => services.viewPreferences.updatePreferences({dismissedNotices: {[dismissalKey('argocd', 'payments', content)]: true}}));

        expect(screen.queryByText(content)).not.toBeInTheDocument();
    });

    it('always shows a permanent notice, without a dismiss button', () => {
        services.viewPreferences.updatePreferences({dismissedNotices: {[dismissalKey('argocd', 'payments', content)]: true}});
        renderBanner({[NoticeAnnotationContent]: content, [NoticeAnnotationPermanent]: 'true'});

        expect(screen.getByText(content)).toBeInTheDocument();
        expect(screen.queryByTitle('Dismiss')).not.toBeInTheDocument();
    });
});
