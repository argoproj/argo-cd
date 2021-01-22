import * as React from 'react';
import {DataLoader} from '../shared/components';
import {services, ViewPreferences} from '../shared/services';
import {Observable} from 'rxjs';
import './ui-banner.scss';

export const Banner = (props: React.Props<any>) => {
    const [visible, setVisible] = React.useState(true);
    return (
        <DataLoader
            load={() =>
                Observable.combineLatest(services.authService.settings(), services.viewPreferences.getPreferences()).map(items => {
                    return {content: items[0].uiBannerContent, url: items[0].uiBannerURL, prefs: items[1]};
                })
            }>
            {({content, url, prefs}: {content: string; url: string; prefs: ViewPreferences}) => {
                const show = visible && prefs.isBannerVisible && content !== undefined;
                return (
                    <React.Fragment>
                        <div className='ui-banner' style={{visibility: show ? 'visible' : 'hidden'}}>
                            <div style={{marginRight: '15px'}}>
                                {url !== undefined ? (
                                    <a href={url} target='_blank'>
                                        {content}
                                    </a>
                                ) : (
                                    <React.Fragment>{content}</React.Fragment>
                                )}
                            </div>
                            <button className='argo-button argo-button--base' style={{marginRight: '5px'}} onClick={() => setVisible(false)}>
                                Dismiss for now
                            </button>
                            <button className='argo-button argo-button--base' onClick={() => services.viewPreferences.updatePreferences({...prefs, isBannerVisible: false})}>
                                Don't show again
                            </button>
                        </div>
                        {show ? <div className='ui-banner--wrapper'>{props.children}</div> : props.children}
                    </React.Fragment>
                );
            }}
        </DataLoader>
    );
};
