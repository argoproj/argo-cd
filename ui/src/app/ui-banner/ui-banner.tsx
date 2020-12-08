import * as React from 'react';
import {DataLoader} from '../shared/components';
import {services} from '../shared/services';

require('./ui-banner.scss');
var crypto = require('crypto');
export class Banner extends React.Component {
    state = {
        bannerVisible: false
    };
    private setBanner() {
        this.setState({
            bannerVisible: false
        });
    }
    private checkBannerUpdate() {
        services.bannerUI.banner().then(response => {
            let nextBannerState = this.createHash(response.announcements.description + response.maintenance.description + response.newRelease.description);
            services.viewPreferences.getPreferences().subscribe(items => {
                let currentBannerState = items.isBannerVisible;
                if (currentBannerState !== nextBannerState) {
                    services.viewPreferences.updatePreferences({
                        isBannerVisible: nextBannerState
                    });
                    this.setState({bannerVisible: true});
                }
            });
        });
    }

    private createHash(val: string): string {
        return crypto
            .createHash('md5')
            .update(val)
            .digest('hex');
    }

    async componentDidMount() {
        await this.checkBannerUpdate();
    }

    render() {
        if (this.state.bannerVisible) {
            return (
                <DataLoader load={() => services.bannerUI.banner()}>
                    {apiData => (
                        <div className='ui_banner' style={{visibility: this.state.bannerVisible ? 'visible' : 'hidden'}}>
                            <button className='ui_banner__close' aria-hidden='true' onClick={this.setBanner}>
                                <span>
                                    <i className='argo-icon-close' aria-hidden='true' />
                                </span>
                            </button>
                            <div className='ui_banner__text'>
                                <div className='ui_banner__items'>
                                    <a href={apiData.announcements['url']} target='_blank' className='ui_banner__items'>
                                        {apiData.announcements['description']}
                                    </a>{' '}
                                </div>
                                <div>
                                    <a href={apiData.maintenance['url']} target='_blank' className='ui_banner__items'>
                                        {apiData.maintenance['description']}
                                    </a>{' '}
                                </div>
                                <div>
                                    <a href={apiData.newRelease['url']} target='_blank' className='ui_banner__items'>
                                        {apiData.newRelease['description']}
                                    </a>{' '}
                                </div>
                            </div>
                        </div>
                    )}
                </DataLoader>
            );
        } else {
            return null;
        }
    }
}
