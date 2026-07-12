import {DataLoader} from 'argo-ui';
import * as React from 'react';
import {Key, KeybindingContext, NumKey, NumKeyToNumber, NumPadKey, useNav} from 'argo-ui/v2';
import {Consumer, Context} from '../../../shared/context';
import * as models from '../../../shared/models';
import * as AppUtils from '../utils';
import {isApp} from '../utils';
import {services} from '../../../shared/services';
import {ApplicationTile} from './application-tile';
import {AppSetTile} from './appset-tile';

import './applications-tiles.scss';

export interface ApplicationTilesProps {
    applications: models.AbstractApplication[];
    syncApplication: (appName: string, appNamespace: string) => any;
    refreshApplication: (appName: string, appNamespace: string) => any;
    deleteApplication: (appName: string, appNamespace: string) => any;
}

const useItemsPerContainer = (itemRef: any, containerRef: any): number => {
    const [itemsPer, setItemsPer] = React.useState(0);

    React.useEffect(() => {
        const handleResize = () => {
            let timeoutId: any;
            clearTimeout(timeoutId);
            timeoutId = setTimeout(() => {
                timeoutId = null;
                const itemWidth = itemRef.current ? itemRef.current.offsetWidth : -1;
                const containerWidth = containerRef.current ? containerRef.current.offsetWidth : -1;
                const curItemsPer = containerWidth > 0 && itemWidth > 0 ? Math.floor(containerWidth / itemWidth) : 1;
                if (curItemsPer !== itemsPer) {
                    setItemsPer(curItemsPer);
                }
            }, 1000);
        };
        window.addEventListener('resize', handleResize);
        handleResize();
        return () => {
            window.removeEventListener('resize', handleResize);
        };
    }, []);

    return itemsPer || 1;
};

export const ApplicationTiles = ({applications, syncApplication, refreshApplication, deleteApplication}: ApplicationTilesProps) => {
    const [selectedApp, navApp, reset] = useNav(applications.length);
    const [collapsedProjects, setCollapsedProjects] = React.useState<Record<string, boolean>>({});

    const getProjName = (app: models.AbstractApplication) => {
        return isApp(app) ? app.spec?.project || 'default' : 'default';
    };

    const groupedApps = React.useMemo(() => {
        const groupsMap = new Map<string, {app: models.AbstractApplication; index: number}[]>();
        applications.forEach((app, i) => {
            const proj = getProjName(app);
            if (!groupsMap.has(proj)) {
                groupsMap.set(proj, []);
            }
            groupsMap.get(proj).push({app, index: i});
        });
        return Array.from(groupsMap.entries()).sort((a, b) => a[0].localeCompare(b[0]));
    }, [applications]);

    const ctxh = React.useContext(Context);
    const firstTileRef = React.useRef<HTMLDivElement>(null);
    const appContainerRef = React.useRef(null);
    const appsPerRow = useItemsPerContainer(firstTileRef, appContainerRef);

    const {registerKeybinding} = React.useContext(KeybindingContext);

    registerKeybinding({keys: Key.RIGHT, action: () => navApp(1)});
    registerKeybinding({keys: Key.LEFT, action: () => navApp(-1)});
    registerKeybinding({keys: Key.DOWN, action: () => navApp(appsPerRow)});
    registerKeybinding({keys: Key.UP, action: () => navApp(-1 * appsPerRow)});

    registerKeybinding({
        keys: Key.ENTER,
        action: () => {
            if (selectedApp > -1) {
                ctxh.navigation.goto(`/${AppUtils.getAppUrl(applications[selectedApp])}`);
                return true;
            }
            return false;
        }
    });

    registerKeybinding({
        keys: Key.ESCAPE,
        action: () => {
            if (selectedApp > -1) {
                reset();
                return true;
            }
            return false;
        }
    });

    registerKeybinding({
        keys: Object.values(NumKey) as NumKey[],
        action: n => {
            reset();
            return navApp(NumKeyToNumber(n));
        }
    });
    registerKeybinding({
        keys: Object.values(NumPadKey) as NumPadKey[],
        action: n => {
            reset();
            return navApp(NumKeyToNumber(n));
        }
    });

    return (
        <Consumer>
            {ctx => (
                <DataLoader load={() => services.viewPreferences.getPreferences()}>
                    {pref => {
                        let firstVisibleIndex = -1;
                        if (pref.appList.groupByProject) {
                            for (const [project, appsInGroup] of groupedApps) {
                                if (!collapsedProjects[project] && appsInGroup.length > 0) {
                                    firstVisibleIndex = appsInGroup[0].index;
                                    break;
                                }
                            }
                        } else {
                            firstVisibleIndex = 0;
                        }

                        return (
                            <div ref={appContainerRef}>
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
                                                    <div className='applications-tiles argo-table-list argo-table-list--clickable'>
                                                        {appsInGroup.map(({app, index}) =>
                                                            isApp(app) ? (
                                                                <ApplicationTile
                                                                    key={AppUtils.appInstanceName(app)}
                                                                    app={app as models.Application}
                                                                    selected={selectedApp === index}
                                                                    pref={pref}
                                                                    ctx={ctx}
                                                                    tileRef={index === firstVisibleIndex ? firstTileRef : undefined}
                                                                    syncApplication={syncApplication}
                                                                    refreshApplication={refreshApplication}
                                                                    deleteApplication={deleteApplication}
                                                                />
                                                            ) : (
                                                                <AppSetTile
                                                                    key={AppUtils.appInstanceName(app)}
                                                                    appSet={app as models.ApplicationSet}
                                                                    selected={selectedApp === index}
                                                                    pref={pref}
                                                                    ctx={ctx}
                                                                    tileRef={index === firstVisibleIndex ? firstTileRef : undefined}
                                                                />
                                                            )
                                                        )}
                                                    </div>
                                                )}
                                            </div>
                                        );
                                    })
                                ) : (
                                    <div className='applications-tiles argo-table-list argo-table-list--clickable'>
                                        {applications.map((app, i) =>
                                            isApp(app) ? (
                                                <ApplicationTile
                                                    key={AppUtils.appInstanceName(app)}
                                                    app={app as models.Application}
                                                    selected={selectedApp === i}
                                                    pref={pref}
                                                    ctx={ctx}
                                                    tileRef={i === 0 ? firstTileRef : undefined}
                                                    syncApplication={syncApplication}
                                                    refreshApplication={refreshApplication}
                                                    deleteApplication={deleteApplication}
                                                />
                                            ) : (
                                                <AppSetTile
                                                    key={AppUtils.appInstanceName(app)}
                                                    appSet={app as models.ApplicationSet}
                                                    selected={selectedApp === i}
                                                    pref={pref}
                                                    ctx={ctx}
                                                    tileRef={i === 0 ? firstTileRef : undefined}
                                                />
                                            )
                                        )}
                                    </div>
                                )}
                            </div>
                        );
                    }}
                </DataLoader>
            )}
        </Consumer>
    );
};
