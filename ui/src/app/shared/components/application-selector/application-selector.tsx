import * as React from 'react';
import {FormFunctionProps} from 'react-form';
import {CheckboxField} from '..';
import * as models from '../../models';
import {ComparisonStatusIcon, HealthStatusIcon, OperationPhaseIcon} from '../../../applications/components/utils';

export const ApplicationSelector = ({apps, formApi}: {apps: models.Application[]; formApi: FormFunctionProps}) => {
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
                    <label key={app.metadata.name} style={{marginTop: '0.5em', cursor: 'pointer'}}>
                        <CheckboxField field={`app/${i}`} />
                        &nbsp;
                        {app.metadata.name}
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
