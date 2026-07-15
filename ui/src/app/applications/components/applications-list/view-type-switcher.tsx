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

    // 1. Add local state to ensure the checkbox updates visually the millisecond it is clicked
    const [isGrouped, setIsGrouped] = React.useState(!!pref.groupByProject);
    const [prevGroupByProject, setPrevGroupByProject] = React.useState(!!pref.groupByProject);

    // 2. Synchronize the local state if the pref prop changes externally during rendering
    if (!!pref.groupByProject !== prevGroupByProject) {
        setIsGrouped(!!pref.groupByProject);
        setPrevGroupByProject(!!pref.groupByProject);
    }

    // Extract clean preferences using rest syntax to prevent type pollution and avoid non-existent fields
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    const {page, search, ...cleanPrefWithoutGroup} = pref;
    const cleanPref: AppsListPreferences = {...cleanPrefWithoutGroup, groupByProject: isGrouped};

    return (
        <div className='applications-list__view-type' style={{display: 'flex', alignItems: 'center'}}>
            <div style={{marginRight: '15px', display: 'flex', alignItems: 'center'}}>
                <Checkbox
                    id='group-by-project-checkbox'
                    checked={isGrouped}
                    onChange={val => {
                        // 4. Update the local checkbox instantly
                        setIsGrouped(val);
                        // 5. Update the persistent backend preferences in the background
                        services.viewPreferences.updatePreferences({
                            appList: {
                                ...cleanPref,
                                groupByProject: val
                            }
                        });
                    }}
                />
                <label htmlFor='group-by-project-checkbox' style={{marginLeft: '6px', cursor: 'pointer', marginBottom: 0}}>
                    Group by Project
                </label>
            </div>
            <i
                className={classNames('fa fa-th', {selected: cleanPref.view === Tiles}, 'menu_icon')}
                title='Tiles'
                onClick={() => {
                    ctx.navigation.goto('.', {view: Tiles});
                    services.viewPreferences.updatePreferences({
                        appList: {...cleanPref, view: Tiles}
                    });
                }}
            />
            <i
                className={classNames('fa fa-th-list', {selected: cleanPref.view === List}, 'menu_icon')}
                title='List'
                onClick={() => {
                    ctx.navigation.goto('.', {view: List});
                    services.viewPreferences.updatePreferences({
                        appList: {...cleanPref, view: List}
                    });
                }}
            />
            <i
                className={classNames('fa fa-chart-pie', {selected: cleanPref.view === Summary}, 'menu_icon')}
                title='Summary'
                onClick={() => {
                    ctx.navigation.goto('.', {view: Summary});
                    services.viewPreferences.updatePreferences({
                        appList: {...cleanPref, view: Summary}
                    });
                }}
            />
        </div>
    );
};
