import * as React from 'react';
import {Checkbox, FormFunctionProps} from 'argo-ui';
import AutoSizer from 'react-virtualized/dist/commonjs/AutoSizer';
import List from 'react-virtualized/dist/commonjs/List';
import * as models from '../../models';
import {appInstanceName, appQualifiedName, ComparisonStatusIcon, HealthStatusIcon, OperationPhaseIcon} from '../app-utils';
import {AuthSettingsCtx} from '../../context';

export const ApplicationSelector = ({apps, formApi}: {apps: models.Application[]; formApi: FormFunctionProps}) => {
    const useAuthSettingsCtx = React.useContext(AuthSettingsCtx);
    const id = React.useId();
    const virtualized = apps.length > 50;
    const selectApps = (isSelected: (app: models.Application) => boolean) => {
        const values = {...formApi.getFormState().values};
        apps.forEach((app, i) => (values[`app/${i}`] = isSelected(app)));
        formApi.setAllValues(values);
    };
    const renderRow = (app: models.Application, index: number, key: string, style?: React.CSSProperties) => (
        <div key={key} style={{...style, display: 'flex', alignItems: 'center', paddingTop: '0.5em'}}>
            <Checkbox id={`${id}-${index}`} checked={!!formApi.values[`app/${index}`]} onChange={checked => formApi.setValue(`app/${index}`, checked)} />
            &nbsp;
            <label htmlFor={`${id}-${index}`} style={{margin: 0, cursor: 'pointer', ...(virtualized ? {whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis'} : {})}}>
                {app.isAppOfAppsPattern ? '(App of Apps) ' : ''}
                {appQualifiedName(app, useAuthSettingsCtx?.appsInAnyNamespaceEnabled)}
            </label>
            &nbsp;
            <ComparisonStatusIcon status={app.status.sync?.status} />
            &nbsp;
            <HealthStatusIcon state={app.status.health} />
            &nbsp;
            <OperationPhaseIcon app={app} />
        </div>
    );
    return (
        <>
            <label>
                Apps (<a onClick={() => selectApps(() => true)}>all</a>/
                <a onClick={() => selectApps(app => app.status.sync?.status === models.SyncStatuses.OutOfSync)}>out of sync</a>/<a onClick={() => selectApps(() => false)}>none</a>
                ):
            </label>
            <div style={{marginTop: '0.4em', ...(virtualized ? {height: '60vh'} : {})}}>
                {virtualized ? (
                    <AutoSizer>
                        {({height, width}) => (
                            <List
                                height={height}
                                width={width}
                                rowCount={apps.length}
                                rowHeight={36}
                                rowRenderer={({index, key, style}) => renderRow(apps[index], index, key, style)}
                            />
                        )}
                    </AutoSizer>
                ) : (
                    apps.map((app, i) => renderRow(app, i, appInstanceName(app)))
                )}
            </div>
        </>
    );
};
