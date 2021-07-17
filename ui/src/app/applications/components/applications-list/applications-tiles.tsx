import {ActionButton, Flexy, useTimeout} from 'argo-ui/v2';
import * as React from 'react';
import {Key, KeybindingContext, NumKey, NumKeyToNumber, NumPadKey, useNav} from 'react-keyhooks';
import {Consumer, Context} from '../../../shared/context';
import * as models from '../../../shared/models';
import {ApplicationTile} from '../application-tile/application-tile';

require('./applications-tiles.scss');

export interface ApplicationTilesProps {
    applications: models.Application[];
    syncApplication: (appName: string) => any;
    refreshApplication: (appName: string | string[]) => any;
    deleteApplication: (appName: string) => any;
    compact: boolean;
    setCompact: (compact: boolean) => void;
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

export const ApplicationTiles = ({applications, syncApplication, refreshApplication, deleteApplication, compact, setCompact}: ApplicationTilesProps) => {
    const [selectedApp, navApp, reset] = useNav(applications.length);

    const ctxh = React.useContext(Context);
    const appRef = {ref: React.useRef(null), set: false};
    const appContainerRef = React.useRef(null);
    const appsPerRow = useItemsPerContainer(appRef.ref, appContainerRef);
    const [checkedApps, setCheckedApps] = React.useState<{[key: string]: boolean}>({});
    const [allSelected, setAllSelected] = React.useState(false);
    const [refreshing, setRefreshing] = React.useState(false);

    useTimeout(() => setRefreshing(false), 1000, [refreshing]);

    React.useEffect(() => {
        setAllSelected((Object.keys(checkedApps) || []).length === (applications || []).length);
    }, [applications, checkedApps]);

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

    const getAppName = (app: models.Application) => {
        return app.metadata ? app.metadata.name : 'Unknown';
    };

    const selectAll = () => {
        const update = {...checkedApps};
        for (const app of applications) {
            const name = getAppName(app);
            if (!update[name]) {
                update[name] = true;
            }
        }
        setCheckedApps(update);
    };

    const deselectAll = () => {
        const update = {...checkedApps};
        for (const app of applications) {
            const name = getAppName(app);
            if (update[name]) {
                delete update[name];
            }
        }
        setCheckedApps(update);
    };

    return (
        <Consumer>
            {ctx => (
                <React.Fragment>
                    <Flexy style={{height: '1em', margin: '1em 0'}}>
                        <span>
                            SHOWING <b>{(applications || []).length}</b> APPS
                            {(Object.keys(checkedApps) || []).length > 0 && ` (${(Object.keys(checkedApps) || []).length} SELECTED)`}
                        </span>
                        <Flexy style={{marginLeft: 'auto', lineHeight: '1em', fontSize: '14px'}}>
                            {(Object.keys(checkedApps) || []).length > 0 && (
                                <React.Fragment>
                                    <ActionButton
                                        action={() => {
                                            setRefreshing(true);
                                            refreshApplication(Object.keys(checkedApps));
                                        }}
                                        label='REFRESH'
                                        icon='fa-redo'
                                        indicateLoading={true}
                                    />
                                </React.Fragment>
                            )}
                            {(Object.keys(checkedApps) || []).length > 0 && <ActionButton label='DESELECT ALL' icon='fa-times' action={deselectAll} />}
                            {!allSelected && <ActionButton label='SELECT ALL' icon='fa-check-double' action={selectAll} />}
                            <ActionButton
                                label={`${compact ? 'NORMAL' : 'COMPACT'} VIEW`}
                                icon={compact ? 'fa-expand' : 'fa-compress'}
                                action={() => {
                                    setCompact(!compact);
                                }}
                                style={{width: '150px'}}
                            />
                        </Flexy>
                    </Flexy>
                    <div className='applications-tiles argo-table-list argo-table-list--clickable row small-up-1 medium-up-2 large-up-3 xxxlarge-up-4' ref={appContainerRef}>
                        {(applications || []).map((app, i) => {
                            const name = getAppName(app);
                            return (
                                <ApplicationTile
                                    key={app.metadata ? app.metadata.name : i}
                                    selected={selectedApp === i}
                                    checked={checkedApps[name]}
                                    ref={appRef.set ? appRef.ref : null}
                                    onSelect={val => {
                                        const update = {...checkedApps};
                                        if (val) {
                                            update[name] = true;
                                        } else if (update[name]) {
                                            delete update[name];
                                        }
                                        setCheckedApps(update);
                                    }}
                                    compact={compact}
                                    app={app}
                                    syncApplication={syncApplication}
                                    deleteApplication={deleteApplication}
                                    refreshApplication={refreshApplication}
                                    refreshing={refreshing && checkedApps[name]}
                                />
                            );
                        })}
                    </div>
                </React.Fragment>
            )}
        </Consumer>
    );
};
