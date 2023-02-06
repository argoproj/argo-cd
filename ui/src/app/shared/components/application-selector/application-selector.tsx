import * as React from 'react';
import {FormFunctionProps} from 'react-form';
import {CheckboxField} from '..';
import * as models from '../../models';
import {appInstanceName, appQualifiedName, ComparisonStatusIcon, HealthStatusIcon, OperationPhaseIcon} from '../../../applications/components/utils';
import {AuthSettingsCtx} from '../../context';

export const ApplicationSelector = ({apps, formApi}: {apps: models.Application[]; formApi: FormFunctionProps}) => {
    const useAuthSettingsCtx = React.useContext(AuthSettingsCtx);
    return (
        <>
            <label>
                Apps (<a onClick={() => apps.forEach((_, i) => formApi.setValue('app/' + i, true))}>all</a>/
                <a onClick={() => apps.forEach((app, i) => formApi.setValue('app/' + i, app.status.sync.status === models.SyncStatuses.OutOfSync))}>out of sync</a>/
                <a onClick={() => apps.forEach((_, i) => formApi.setValue('app/' + i, false))}>none</a>
                ):
            </label>
            <div style={{marginTop: '0.4em'}}>
                {apps.map((app, i) => (
                    <label key={appInstanceName(app)} style={{marginTop: '0.5em', cursor: 'pointer'}}>
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
