import {Tooltip} from 'argo-ui';
import * as classNames from 'classnames';
import * as React from 'react';
import {ContextApis} from '../../../shared/context';
import {AppsListPreferences, AppsListViewKey, HealthStatusBarPreferences, services} from '../../../shared/services';

interface ViewTypeSwitcherProps {
    pref: AppsListPreferences & {page: number; search: string};
    ctx: ContextApis;
    healthBarPrefs: HealthStatusBarPreferences;
}

export const ViewTypeSwitcher: React.FC<ViewTypeSwitcherProps> = ({pref, ctx, healthBarPrefs}) => {
    const {List, Summary, Tiles} = AppsListViewKey;

    return (
        <React.Fragment>
            <Tooltip content='Toggle Health Status Bar'>
                <button
                    className={`applications-list__accordion argo-button argo-button--base${healthBarPrefs.showHealthStatusBar ? '-o' : ''}`}
                    style={{border: 'none'}}
                    onClick={() => {
                        healthBarPrefs.showHealthStatusBar = !healthBarPrefs.showHealthStatusBar;
                        services.viewPreferences.updatePreferences({
                            appList: {
                                ...pref,
                                statusBarView: {
                                    ...healthBarPrefs,
                                    showHealthStatusBar: healthBarPrefs.showHealthStatusBar
                                }
                            }
                        });
                    }}>
                    <i className='fas fa-ruler-horizontal' />
                </button>
            </Tooltip>
            <div className='applications-list__view-type' style={{marginLeft: 'auto'}}>
                <i
                    className={classNames('fa fa-th', {selected: pref.view === Tiles}, 'menu_icon')}
                    title='Tiles'
                    onClick={() => {
                        ctx.navigation.goto('.', {view: Tiles});
                        services.viewPreferences.updatePreferences({appList: {...pref, view: Tiles}});
                    }}
                />
                <i
                    className={classNames('fa fa-th-list', {selected: pref.view === List}, 'menu_icon')}
                    title='List'
                    onClick={() => {
                        ctx.navigation.goto('.', {view: List});
                        services.viewPreferences.updatePreferences({appList: {...pref, view: List}});
                    }}
                />
                <i
                    className={classNames('fa fa-chart-pie', {selected: pref.view === Summary}, 'menu_icon')}
                    title='Summary'
                    onClick={() => {
                        ctx.navigation.goto('.', {view: Summary});
                        services.viewPreferences.updatePreferences({appList: {...pref, view: Summary}});
                    }}
                />
            </div>
        </React.Fragment>
    );
};
