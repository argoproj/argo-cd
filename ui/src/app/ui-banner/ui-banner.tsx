import {Tooltip} from 'argo-ui';
import * as React from 'react';
import {combineLatest} from 'rxjs';
import {map} from 'rxjs/operators';
import {ExternalLink} from '../applications/components/application-urls';

import {DataLoader} from '../shared/components';
import {services, ViewPreferences} from '../shared/services';
import './ui-banner.scss';

const CustomBanner = (props: {
    combinedBannerClassName: string;
    show: boolean;
    heightOfBanner: string;
    leftOffset: string;
    permanent: boolean;
    url: string;
    content: string;
    setVisible: (visible: boolean) => void;
    prefs: object;
}) => {
    return (
        <div className={props.combinedBannerClassName} style={{visibility: props.show ? 'visible' : 'hidden', height: props.heightOfBanner, marginLeft: props.leftOffset}}>
            <div className='ui-banner-text' style={{maxHeight: props.permanent ? '25px' : '50px'}}>
                {props.url !== undefined ? (
                    <a href={props.url} target='_blank' rel='noopener noreferrer'>
                        {props.content}
                    </a>
                ) : (
                    <React.Fragment>{props.content}</React.Fragment>
                )}
            </div>
            {!props.permanent ? (
                <>
                    <button className='ui-banner-button argo-button argo-button--base' style={{marginRight: '5px'}} onClick={() => props.setVisible(false)}>
                        Dismiss for now
                    </button>
                    <button
                        className='ui-banner-button argo-button argo-button--base'
                        onClick={() => services.viewPreferences.updatePreferences({...props.prefs, hideBannerContent: props.content})}>
                        Don't show again
                    </button>
                </>
            ) : (
                <></>
            )}
        </div>
    );
};

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
                let chatBottomPosition = 10;
                if (show && (!isTop || position === 'both')) {
                    if (permanent) {
                        chatBottomPosition = 40;
                    } else {
                        chatBottomPosition = 85;
                    }
                }
                if (chatUrl) {
                    try {
                        const externalLink = new ExternalLink(chatUrl);
                        chatUrl = externalLink.ref;
                    } catch (InvalidExternalLinkError) {
                        chatUrl = 'invalid-url';
                    }
                }
                const shouldRenderTop = position === 'top' || position === 'both' || (!position && content);
                const shouldRenderBottom = position === 'bottom' || position === 'both';
                return (
                    <React.Fragment>
                        {shouldRenderTop && (
                            <CustomBanner
                                combinedBannerClassName={'ui-banner ui-banner-top'}
                                show={show}
                                heightOfBanner={heightOfBanner}
                                leftOffset={leftOffset}
                                permanent={permanent}
                                url={url}
                                content={content}
                                setVisible={setVisible}
                                prefs={prefs}
                            />
                        )}
                        {shouldRenderBottom && (
                            <CustomBanner
                                combinedBannerClassName={'ui-banner ui-banner-bottom'}
                                show={show}
                                heightOfBanner={heightOfBanner}
                                leftOffset={leftOffset}
                                permanent={permanent}
                                url={url}
                                content={content}
                                setVisible={setVisible}
                                prefs={prefs}
                            />
                        )}
                        {show ? <div className={wrapperClassname}>{props.children}</div> : props.children}
                        {chatUrl && (
                            <div style={{position: 'fixed', right: 10, bottom: chatBottomPosition}}>
                                {chatUrl === 'invalid-url' ? (
                                    <Tooltip content='Invalid URL provided'>
                                        <a className='argo-button disabled argo-button--special'>
                                            <i className='fas fa-comment-alt' /> {chatText}
                                        </a>
                                    </Tooltip>
                                ) : (
                                    <a href={chatUrl} className='argo-button argo-button--special'>
                                        <i className='fas fa-comment-alt' /> {chatText}
                                    </a>
                                )}
                            </div>
                        )}
                    </React.Fragment>
                );
            }}
        </DataLoader>
    );
};
