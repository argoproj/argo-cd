import * as React from 'react';
import {combineLatest} from 'rxjs';
import {map} from 'rxjs/operators';

import {DataLoader} from '../shared/components';
import {services, ViewPreferences} from '../shared/services';
import './ui-banner.scss';

export const Banner = (props: React.Props<any>) => {
    const [visible, setVisible] = React.useState(true);
    return (
        <DataLoader
            load={() =>
                combineLatest([services.authService.settings(), services.viewPreferences.getPreferences()]).pipe(
                    map(items => {
                        return {
                            content: items[0].uiBannerContent,
                            url: items[0].uiBannerURL,
                            prefs: items[1],
                            permanent: items[0].uiBannerPermanent,
                            chatText: items[0].help.chatText,
                            chatUrl: items[0].help.chatUrl,
                            position: items[0].uiBannerPosition
                        };
                    })
                )
            }>
            {({
                content,
                url,
                prefs,
                permanent,
                chatText,
                chatUrl,
                position
            }: {
                content: string;
                url: string;
                prefs: ViewPreferences;
                permanent: boolean;
                chatText: string;
                chatUrl: string;
                position: string;
            }) => {
                const heightOfBanner = permanent ? '28px' : '70px';
                const leftOffset = prefs.hideSidebar ? '60px' : '230px';
                let show = false;
                if (!content || content === '' || content === null) {
                    if (prefs.hideBannerContent) {
                        services.viewPreferences.updatePreferences({...prefs, hideBannerContent: null});
                    }
                } else {
                    if (prefs.hideBannerContent !== content) {
                        show = true;
                    }
                }
                show = permanent || (show && visible);
                const isTop = position !== 'bottom';
                const bannerClassName = isTop ? 'ui-banner-top' : 'ui-banner-bottom';
                const wrapperClassname = bannerClassName + '--wrapper ' + (!permanent ? bannerClassName + '--wrapper-multiline' : bannerClassName + '--wrapper-singleline');
                const combinedBannerClassName = isTop ? 'ui-banner ui-banner-top' : 'ui-banner ui-banner-bottom';
                let chatBottomPosition = 10;
                if (show && !isTop) {
                    if (permanent) {
                        chatBottomPosition = 40;
                    } else {
                        chatBottomPosition = 85;
                    }
                }
                return (
                    <React.Fragment>
                        <div className={combinedBannerClassName} style={{visibility: show ? 'visible' : 'hidden', height: heightOfBanner, left: leftOffset}}>
                            <div className='ui-banner-text' style={{maxHeight: permanent ? '25px' : '50px'}}>
                                {url !== undefined ? (
                                    <a href={url} target='_blank' rel='noopener noreferrer'>
                                        {content}
                                    </a>
                                ) : (
                                    <React.Fragment>{content}</React.Fragment>
                                )}
                            </div>
                            {!permanent ? (
                                <>
                                    <button className='ui-banner-button argo-button argo-button--base' style={{marginRight: '5px'}} onClick={() => setVisible(false)}>
                                        Dismiss for now
                                    </button>
                                    <button
                                        className='ui-banner-button argo-button argo-button--base'
                                        onClick={() => services.viewPreferences.updatePreferences({...prefs, hideBannerContent: content})}>
                                        Don't show again
                                    </button>
                                </>
                            ) : (
                                <></>
                            )}
                        </div>
                        {show ? <div className={wrapperClassname}>{props.children}</div> : props.children}
                        {chatUrl && (
                            <div style={{position: 'fixed', right: 10, bottom: chatBottomPosition}}>
                                <a href={chatUrl} className='argo-button argo-button--special'>
                                    <i className='fas fa-comment-alt' /> {chatText}
                                </a>
                            </div>
                        )}
                    </React.Fragment>
                );
            }}
        </DataLoader>
    );
};
