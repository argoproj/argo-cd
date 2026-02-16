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

    const ctxh = React.useContext(Context);
    const firstTileRef = React.useRef<HTMLDivElement>(null);
    const appContainerRef = React.useRef(null);
    const appsPerRow = useItemsPerContainer(firstTileRef, appContainerRef);

    const {useKeybinding} = React.useContext(KeybindingContext);

    useKeybinding({keys: Key.RIGHT, action: () => navApp(1)});
    useKeybinding({keys: Key.LEFT, action: () => navApp(-1)});
    useKeybinding({keys: Key.DOWN, action: () => navApp(appsPerRow)});
    useKeybinding({keys: Key.UP, action: () => navApp(-1 * appsPerRow)});

    useKeybinding({
        keys: Key.ENTER,
        action: () => {
            if (selectedApp > -1) {
                ctxh.navigation.goto(`/${AppUtils.getAppUrl(applications[selectedApp])}`);
                return true;
            }
            return false;
        }
    });

    useKeybinding({
        keys: Key.ESCAPE,
        action: () => {
            if (selectedApp > -1) {
                reset();
                return true;
            }
            return false;
        }
    });

    useKeybinding({
        keys: Object.values(NumKey) as NumKey[],
        action: n => {
            reset();
            return navApp(NumKeyToNumber(n));
        }
    });
    useKeybinding({
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
                    {pref => (
                        <div className='applications-tiles argo-table-list argo-table-list--clickable' ref={appContainerRef}>
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
                </DataLoader>
            )}
        </Consumer>
    );
};
