import classNames from 'classnames';
import * as React from 'react';
import {Checkbox} from 'argo-ui';
import {ContextApis} from '../../../shared/context';
import {AppsListPreferences, AppsListViewKey, services} from '../../../shared/services';

interface ViewTypeSwitcherProps {
    pref: AppsListPreferences & {page: number; search: string};
    ctx: ContextApis;
}

export const ViewTypeSwitcher: React.FC<ViewTypeSwitcherProps> = ({pref, ctx}) => {
    const {List, Summary, Tiles} = AppsListViewKey;

    return (
        <div className='applications-list__view-type' style={{display: 'flex', alignItems: 'center'}}>
            <div style={{marginRight: '15px', display: 'flex', alignItems: 'center'}}>
                <Checkbox
                    id='group-by-project-checkbox'
                    checked={!!pref.groupByProject}
                    onChange={val => {
                        services.viewPreferences.updatePreferences({appList: {...pref, groupByProject: val}});
                    }}
                />
                <label htmlFor='group-by-project-checkbox' style={{marginLeft: '6px', cursor: 'pointer', marginBottom: 0}}>
                    Group by Project
                </label>
            </div>
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
    );
};
