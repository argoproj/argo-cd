import {DataLoader} from 'argo-ui';
import * as React from 'react';
import {Key, KeybindingContext, useNav} from 'argo-ui/v2';
import {Consumer, Context} from '../../../shared/context';
import * as models from '../../../shared/models';
import * as AppUtils from '../utils';
import {isApp} from '../utils';
import {services} from '../../../shared/services';
import {ApplicationTableRow} from './application-table-row';
import {AppSetTableRow} from './appset-table-row';

import './applications-table.scss';

export const ApplicationsTable = (props: {
    applications: models.AbstractApplication[];
    syncApplication: (appName: string, appNamespace: string) => any;
    refreshApplication: (appName: string, appNamespace: string) => any;
    deleteApplication: (appName: string, appNamespace: string) => any;
}) => {
    const [selectedApp, navApp, reset] = useNav(props.applications.length);
    const [collapsedProjects, setCollapsedProjects] = React.useState<Record<string, boolean>>({});

    const getProjName = (app: models.AbstractApplication) => {
        return isApp(app) ? app.spec?.project || 'default' : 'default';
    };

    const groupedApps = React.useMemo(() => {
        const groupsMap = new Map<string, {app: models.AbstractApplication; index: number}[]>();
        props.applications.forEach((app, i) => {
            const proj = getProjName(app);
            if (!groupsMap.has(proj)) {
                groupsMap.set(proj, []);
            }
            groupsMap.get(proj).push({app, index: i});
        });
        return Array.from(groupsMap.entries()).sort((a, b) => a[0].localeCompare(b[0]));
    }, [props.applications]);

    const ctxh = React.useContext(Context);

    const {registerKeybinding} = React.useContext(KeybindingContext);

    registerKeybinding({keys: Key.DOWN, action: () => navApp(1)});
    registerKeybinding({keys: Key.UP, action: () => navApp(-1)});
    registerKeybinding({
        keys: Key.ESCAPE,
        action: () => {
            reset();
            return selectedApp > -1 ? true : false;
        }
    });
    registerKeybinding({
        keys: Key.ENTER,
        action: () => {
            if (selectedApp > -1) {
                ctxh.navigation.goto(`/${AppUtils.getAppUrl(props.applications[selectedApp])}`);
                return true;
            }
            return false;
        }
    });

    return (
        <Consumer>
            {ctx => (
                <DataLoader load={() => services.viewPreferences.getPreferences()}>
                    {pref => (
                        <div>
                            {pref.appList.groupByProject ? (
                                groupedApps.map(([project, appsInGroup]) => {
                                    const isCollapsed = !!collapsedProjects[project];
                                    return (
                                        <div key={project} className='project-group'>
                                            <div className='project-group__header' onClick={() => setCollapsedProjects({...collapsedProjects, [project]: !isCollapsed})}>
                                                <i className={`fa fa-angle-${isCollapsed ? 'right' : 'down'}`} />
                                                <span className='project-group__header__title'>{project}</span>
                                                <span className='project-group__header__count'>{appsInGroup.length}</span>
                                            </div>
                                            {!isCollapsed && (
                                                <div className='applications-table argo-table-list argo-table-list--clickable'>
                                                    {appsInGroup.map(({app, index}) =>
                                                        isApp(app) ? (
                                                            <ApplicationTableRow
                                                                key={AppUtils.appInstanceName(app)}
                                                                app={app as models.Application}
                                                                selected={selectedApp === index}
                                                                pref={pref}
                                                                ctx={ctx}
                                                                syncApplication={props.syncApplication}
                                                                refreshApplication={props.refreshApplication}
                                                                deleteApplication={props.deleteApplication}
                                                            />
                                                        ) : (
                                                            <AppSetTableRow
                                                                key={AppUtils.appInstanceName(app)}
                                                                appSet={app as models.ApplicationSet}
                                                                selected={selectedApp === index}
                                                                pref={pref}
                                                                ctx={ctx}
                                                            />
                                                        )
                                                    )}
                                                </div>
                                            )}
                                        </div>
                                    );
                                })
                            ) : (
                                <div className='applications-table argo-table-list argo-table-list--clickable'>
                                    {props.applications.map((app, i) =>
                                        isApp(app) ? (
                                            <ApplicationTableRow
                                                key={AppUtils.appInstanceName(app)}
                                                app={app as models.Application}
                                                selected={selectedApp === i}
                                                pref={pref}
                                                ctx={ctx}
                                                syncApplication={props.syncApplication}
                                                refreshApplication={props.refreshApplication}
                                                deleteApplication={props.deleteApplication}
                                            />
                                        ) : (
                                            <AppSetTableRow
                                                key={AppUtils.appInstanceName(app)}
                                                appSet={app as models.ApplicationSet}
                                                selected={selectedApp === i}
                                                pref={pref}
                                                ctx={ctx}
                                            />
                                        )
                                    )}
                                </div>
                            )}
                        </div>
                    )}
                </DataLoader>
            )}
        </Consumer>
    );
};
