import {useState} from 'react';
import * as React from 'react';
import {Page} from '../../../shared/components';

require('./ui-settings-list.scss');

export const UiSettingsList: React.FunctionComponent = () => {
    const [theme, setTheme] = useState(localStorage.getItem('theme') || 'light');

    return (
        <Page
            title={'Ui Settings'}
            toolbar={{
                breadcrumbs: [{title: 'Settings', path: '/settings'}, {title: 'Ui Settings'}]
            }}>
            <div className='ui-settings-list'>
                <div className='argo-container'>
                    <div className='ui-settings-list__panel'>
                        <div className='columns'>Dark Mode (beta)</div>
                        <div className='columns'>
                            <button
                                className='argo-button argo-button--base ui-settings-list__button'
                                onClick={() => {
                                    const targetTheme = theme === 'light' ? 'dark' : 'light';
                                    setTheme(targetTheme);
                                    localStorage.setItem('theme', targetTheme);
                                }}>
                                {theme === 'light' ? 'Enable' : 'Disable'}
                            </button>
                        </div>
                    </div>
                </div>
            </div>
        </Page>
    );
};
