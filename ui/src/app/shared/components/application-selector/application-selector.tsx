import * as React from 'react';
import {FormFunctionProps} from 'react-form';
import {CheckboxField} from '..';
import * as models from '../../models';
import {appInstanceName, appQualifiedName, ComparisonStatusIcon, HealthStatusIcon, OperationPhaseIcon} from '../../../applications/components/utils';
import {AuthSettingsCtx} from '../../context';

export const ApplicationSelector = ({apps, formApi}: {apps: models.Application[]; formApi: FormFunctionProps}) => {
    const reorderedAppList: models.Application[] = [];
    for (const application of apps) {
        if (application.isAppOfAppsPattern) {
            reorderedAppList.push(application);
            for (const childAppName of application.childApps) {
                const indexOfChild = apps.findIndex(app => app.metadata.name === childAppName);
                if (indexOfChild > -1 && reorderedAppList.findIndex(app => app.metadata.name === childAppName) < 0) {
                    reorderedAppList.push(apps[indexOfChild]);
                }
            }
        } else if (!application.isAppOfAppsPattern && !application.parentApp) {
            reorderedAppList.push(application);
        }
    }

    const useAuthSettingsCtx = React.useContext(AuthSettingsCtx);
    return (
        <>
            <label>
                Apps (<a onClick={() => reorderedAppList.forEach((_, i) => formApi.setValue('app/' + i, true))}>all</a>/
                <a onClick={() => reorderedAppList.forEach((app, i) => formApi.setValue('app/' + i, app.status.sync.status === models.SyncStatuses.OutOfSync))}>out of sync</a>/
                <a onClick={() => reorderedAppList.forEach((_, i) => formApi.setValue('app/' + i, false))}>none</a>
                ):
            </label>
            <div style={{marginTop: '0.4em'}}>
                {reorderedAppList.map((app, i) => (
                    <label key={appInstanceName(app)} style={{marginTop: '0.5em', cursor: 'pointer', marginLeft: app.parentApp ? '1rem' : 0}}>
                        <CheckboxField field={`app/${i}`} />
                        &nbsp;
                        {app.isAppOfAppsPattern
                            ? `(App of Apps) ${appQualifiedName(app, useAuthSettingsCtx?.appsInAnyNamespaceEnabled)}`
                            : appQualifiedName(app, useAuthSettingsCtx?.appsInAnyNamespaceEnabled)}
                        &nbsp;
                        <ComparisonStatusIcon status={app.status.sync.status} />
                        &nbsp;
                        <HealthStatusIcon state={app.status.health} />
                        &nbsp;
                        <OperationPhaseIcon app={app} />
                        <br />
                    </label>
                ))}
            </div>
        </>
    );
};
