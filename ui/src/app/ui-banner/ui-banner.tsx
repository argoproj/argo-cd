import * as React from 'react';
import {DataLoader} from '../shared/components';
import {services} from '../shared/services';

require('./ui-banner.scss');
export class Banner extends React.Component {
    state = {
        bannerVisible: false,
        currentBannerState: '',
        nextBannerState: ''
    };
    setBanner = () => {
        this.setState({
            bannerVisible: false
        });
    };
    checkBannerUpdate = () => {
        services.bannerUI.banner().then(response => {
            this.setState({nextBannerState: response.announcements.description + response.maintenance.description + response.newRelease.description});
            services.viewPreferences.getPreferences().subscribe(items => {
                this.setState({currentBannerState: items.isBannerVisible});
            });
            if (this.state.currentBannerState !== this.state.nextBannerState) {
                services.viewPreferences.updatePreferences({isBannerVisible: this.state.nextBannerState});
                this.setState({bannerVisible: true});
            }
        });
    };
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
                                <div>
                                    <a href={apiData.announcements['url']} className='ui_banner__items'>
                                        {apiData.announcements['description']}
                                    </a>{' '}
                                </div>
                                <div>
                                    <a href={apiData.maintenance['url']} className='ui_banner__items'>
                                        {apiData.maintenance['description']}
                                    </a>{' '}
                                </div>
                                <div>
                                    <a href={apiData.newRelease['url']} className='ui_banner__items'>
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
