import * as React from 'react';
import {DataLoader, Page} from '../../../shared/components';
import {services} from '../../../shared/services';
import {Select, SelectOption} from 'argo-ui';

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
                                <div className='row'>
                                    <span>Dark Theme</span>
                                    <Select
                                        value={pref.theme}
                                        onChange={(value: SelectOption) => services.viewPreferences.updatePreferences({theme: value.value})}
                                        options={[
                                            {value: 'auto', title: 'Auto'},
                                            {value: 'light', title: 'Light'},
                                            {value: 'dark', title: 'Dark'}
                                        ]}></Select>
                                </div>
                            </div>
                        </div>
                    </div>
                )}
            </DataLoader>
        </Page>
    );
};
