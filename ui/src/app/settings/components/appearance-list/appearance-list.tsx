import * as React from 'react';
import {DataLoader, Page} from '../../../shared/components';
import {services} from '../../../shared/services';

require('./appearance-list.scss');

export const AppearanceList = () => {
    return (
        <Page
            title={'Appearance'}
            toolbar={{
                breadcrumbs: [{title: 'Settings', path: '/settings'}, {title: 'Appearance'}]
            }}>
            <DataLoader load={() => services.viewPreferences.getPreferences()}>
                {pref => (
                    <div className='appearance-list'>
                        <div className='argo-container'>
                            <div className='appearance-list__panel'>
                                <div className='columns'>System Theme</div>
                                <div className='columns'>
                                    <button
                                        className='argo-button argo-button--base appearance-list__button'
                                        onClick={() => {
                                            services.viewPreferences.updatePreferences({useSystemTheme: !pref.useSystemTheme});
                                        }}>
                                        {pref.useSystemTheme ? 'Disable' : 'Enable'}
                                    </button>
                                </div>
                            </div>
                        </div>
                        <div className='argo-container'>
                            <div className='appearance-list__panel'>
                                <div className='columns'>Dark Theme</div>
                                <div className='columns'>
                                    <button
                                        className={`argo-button ${pref.useSystemTheme && 'disabled'} argo-button--base appearance-list__button`}
                                        onClick={() => {
                                            if (!pref.useSystemTheme) {
                                                const targetTheme = pref.selectedTheme === 'light' ? 'dark' : 'light';
                                                services.viewPreferences.updatePreferences({selectedTheme: targetTheme});
                                            }
                                        }}>
                                        {pref.selectedTheme === 'light' ? 'Enable' : 'Disable'}
                                    </button>
                                </div>
                            </div>
                        </div>
                    </div>
                )}
            </DataLoader>
        </Page>
    );
};
