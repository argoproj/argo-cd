import * as React from 'react';
import {Key, KeybindingContext, NumKey, NumKeyToNumber, NumPadKey, useNav} from 'react-keyhooks';
import {Consumer, Context} from '../../../shared/context';
import * as models from '../../../shared/models';
import {ApplicationTile} from '../application-tile/application-tile';

require('./applications-tiles.scss');

export interface ApplicationTilesProps {
    applications: models.Application[];
    syncApplication: (appName: string) => any;
    refreshApplication: (appName: string) => any;
    deleteApplication: (appName: string) => any;
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
    const appRef = {ref: React.useRef(null), set: false};
    const appContainerRef = React.useRef(null);
    const appsPerRow = useItemsPerContainer(appRef.ref, appContainerRef);
    const [checkedApps, setCheckedApps] = React.useState<{[key: string]: models.Application}>({});

    const {useKeybinding} = React.useContext(KeybindingContext);

    useKeybinding(Key.RIGHT, () => navApp(1));
    useKeybinding(Key.LEFT, () => navApp(-1));
    useKeybinding(Key.DOWN, () => navApp(appsPerRow));
    useKeybinding(Key.UP, () => navApp(-1 * appsPerRow));

    useKeybinding(Key.ENTER, () => {
        if (selectedApp > -1) {
            ctxh.navigation.goto(`/applications/${applications[selectedApp].metadata.name}`);
            return true;
        }
        return false;
    });

    useKeybinding(Key.ESCAPE, () => {
        if (selectedApp > -1) {
            reset();
            return true;
        }
        return false;
    });

    useKeybinding(Object.values(NumKey) as NumKey[], n => {
        reset();
        return navApp(NumKeyToNumber(n));
    });
    useKeybinding(Object.values(NumPadKey) as NumPadKey[], n => {
        reset();
        return navApp(NumKeyToNumber(n));
    });

    return (
        <Consumer>
            {ctx => (
                <React.Fragment>
                    <div style={{height: '1em'}}>{(Object.keys(checkedApps) || []).length > 0 && `Batch Actions (${(Object.keys(checkedApps) || []).length})`}</div>
                    <div className='applications-tiles argo-table-list argo-table-list--clickable row small-up-1 medium-up-2 large-up-3 xxxlarge-up-4' ref={appContainerRef}>
                        {applications.map((app, i) => {
                            const name = app.metadata ? app.metadata.name : 'Unknown';
                            return (
                                <ApplicationTile
                                    key={app.metadata ? app.metadata.name : i}
                                    selected={selectedApp === i}
                                    ref={appRef.set ? appRef.ref : null}
                                    onSelect={val => {
                                        const update = {...checkedApps};
                                        if (val) {
                                            update[name] = app;
                                        } else if (update[name]) {
                                            delete update[name];
                                        }
                                        setCheckedApps(update);
                                    }}
                                    app={app}
                                    syncApplication={syncApplication}
                                    deleteApplication={deleteApplication}
                                    refreshApplication={refreshApplication}
                                />
                            );
                        })}
                    </div>
                </React.Fragment>
            )}
        </Consumer>
    );
};
