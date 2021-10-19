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
                        return {content: items[0].uiBannerContent, url: items[0].uiBannerURL, prefs: items[1], permanent: items[0].uiBannerPermanent};
                    })
                )
            }>
            {({content, url, prefs, permanent}: {content: string; url: string; prefs: ViewPreferences; permanent: boolean}) => {
                const heightOfBanner = permanent ? '28px' : '70px';
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
                const wrapperClassname = 'ui-banner--wrapper ' + (!permanent ? 'ui-banner--wrapper-multiline' : 'ui-banner--wrapper-singleline');
                return (
                    <React.Fragment>
                        <div className='ui-banner' style={{visibility: show ? 'visible' : 'hidden', height: heightOfBanner}}>
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
                        {show ? (
                            <div className={wrapperClassname} style={{marginTop: heightOfBanner}}>
                                {props.children}
                            </div>
                        ) : (
                            props.children
                        )}
                    </React.Fragment>
                );
            }}
        </DataLoader>
    );
};
