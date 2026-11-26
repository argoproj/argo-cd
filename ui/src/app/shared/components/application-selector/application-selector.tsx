import * as React from 'react';
import {Checkbox, FormFunctionProps} from 'argo-ui';
import AutoSizer from 'react-virtualized/dist/commonjs/AutoSizer';
import List from 'react-virtualized/dist/commonjs/List';
import * as models from '../../models';
import {appInstanceName, appQualifiedName, ComparisonStatusIcon, HealthStatusIcon, OperationPhaseIcon} from '../app-utils';
import {AuthSettingsCtx} from '../../context';

// Selected apps are tracked by identity rather than by list index: the apps list is updated live
// (new apps are prepended, deleted apps removed), so indexes would point to different apps.
export const getSelectedApps = <T extends models.AbstractApplication>(apps: T[], values: {selectedApps?: string[]}): T[] => {
    const selected = new Set(values.selectedApps || []);
    return apps.filter(app => selected.has(appInstanceName(app)));
};

export const ApplicationSelector = ({apps, formApi}: {apps: models.Application[]; formApi: FormFunctionProps}) => {
    const useAuthSettingsCtx = React.useContext(AuthSettingsCtx);
    const selected = new Set<string>(formApi.values.selectedApps || []);
    const setSelected = (keys: string[]) => formApi.setValue('selectedApps', keys);
    const toggle = (key: string, checked: boolean) => {
        const current: string[] = (formApi.getFormState().values.selectedApps || []).filter((item: string) => item !== key);
        setSelected(checked ? [...current, key] : current);
    };
    const virtualized = apps.length > 50;
    const renderRow = (app: models.Application, rowKey: string, style?: React.CSSProperties) => {
        const key = appInstanceName(app);
        const name = app.isAppOfAppsPattern
            ? `(App of Apps) ${appQualifiedName(app, useAuthSettingsCtx?.appsInAnyNamespaceEnabled)}`
            : appQualifiedName(app, useAuthSettingsCtx?.appsInAnyNamespaceEnabled);
        return (
            <label
                key={rowKey}
                style={
                    virtualized ? {...style, display: 'flex', alignItems: 'center', margin: 0, paddingTop: '0.5em', cursor: 'pointer'} : {marginTop: '0.5em', cursor: 'pointer'}
                }>
                <Checkbox checked={selected.has(key)} onChange={checked => toggle(key, checked)} />
                &nbsp;
                {virtualized ? (
                    <span title={name} style={{whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis'}}>
                        {name}
                    </span>
                ) : (
                    name
                )}
                &nbsp;
                <ComparisonStatusIcon status={app.status.sync?.status} />
                &nbsp;
                <HealthStatusIcon state={app.status.health} />
                &nbsp;
                <OperationPhaseIcon app={app} />
                {!virtualized && <br />}
            </label>
        );
    };
    return (
        <>
            <label>
                Apps (<a onClick={() => setSelected(apps.map(appInstanceName))}>all</a>/
                <a onClick={() => setSelected(apps.filter(app => app.status.sync?.status === models.SyncStatuses.OutOfSync).map(appInstanceName))}>out of sync</a>/
                <a onClick={() => setSelected([])}>none</a>
                ):
            </label>
            <div style={{marginTop: '0.4em', ...(virtualized ? {height: '60vh'} : {})}}>
                {virtualized ? (
                    <AutoSizer>
                        {({height, width}) => (
                            <List height={height} width={width} rowCount={apps.length} rowHeight={36} rowRenderer={({index, key, style}) => renderRow(apps[index], key, style)} />
                        )}
                    </AutoSizer>
                ) : (
                    apps.map(app => renderRow(app, appInstanceName(app)))
                )}
            </div>
        </>
    );
};
