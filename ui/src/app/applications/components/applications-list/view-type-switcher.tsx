import classNames from 'classnames';
import * as React from 'react';
import {ContextApis} from '../../../shared/context';
import {AppsListPreferences, AppsListViewKey, services} from '../../../shared/services';

interface ViewTypeSwitcherProps {
    pref: AppsListPreferences & {page: number; search: string};
    ctx: ContextApis;
    groupByProject?: {enabled: boolean; onToggle: () => void};
}

export const ViewTypeSwitcher: React.FC<ViewTypeSwitcherProps> = ({pref, ctx, groupByProject}) => {
    const {List, Summary, Tiles} = AppsListViewKey;

    return (
        <div className='applications-list__view-type'>
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
            {groupByProject && pref.view !== Summary && (
                <button
                    type='button'
                    className={classNames({selected: groupByProject.enabled}, 'applications-list__group-toggle')}
                    title='Group by project'
                    aria-label='Group by project'
                    aria-pressed={groupByProject.enabled}
                    onClick={groupByProject.onToggle}>
                    <i className='fa fa-layer-group menu_icon' />
                </button>
            )}
        </div>
    );
};
