import {Duration, Ticker} from 'argo-ui';
import * as moment from 'moment';
import * as PropTypes from 'prop-types';
import * as React from 'react';

import {Revision, Timestamp} from '../../../shared/components';
import * as models from '../../../shared/models';

import './application-hydrate-operation-state.scss';

interface Props {
    hydrateOperationState?: models.HydrateOperation;
    /** When true, show troubleshooting copy for spec.sourceHydrator set but no status.sourceHydrator.currentOperation yet. */
    waitingForController?: boolean;
    /** API server reports hydrator disabled while the Application still uses spec.sourceHydrator (status may be residual or reflect a controller with hydrator still enabled). */
    serverHydratorDisabledAtAPI?: boolean;
}

const ServerHydratorDisabledAtAPIExplanation = ({hasHydrateOperationDetails}: {hasHydrateOperationDetails: boolean}) => (
    <div className='application-hydrate-operation-state__server-disabled-notice'>
        <p className='application-hydrate-operation-state__server-disabled-notice-title'>
            <i className='fa fa-exclamation-triangle' /> Source hydrator disabled on the Argo CD API server
        </p>
        <p>
            The Argo CD API server reports Source Hydrator as disabled. The application controller independently loads the hydrator flag and may be enabled even when the API server
            has it disabled.
        </p>
        {hasHydrateOperationDetails && (
            <p>
                The hydrate operation details below <strong>may be a residual status</strong> from an earlier reconciliation (for example from when hydrator was enabled or from
                another environment), or they may reflect live work if the application controller still has hydrator enabled. Sync status and manifests may be stale relative to the
                dry source when server and controller configuration do not match.
            </p>
        )}
        <p>
            To fix this, either enable source hydrator on the API server and application controller consistently (for example <code>hydrator.enabled</code> in the{' '}
            <code>argocd-cmd-params-cm</code> ConfigMap, then restart the relevant workloads), or remove <code>spec.sourceHydrator</code> from the Application spec if you do not
            intend to use the feature.
        </p>
    </div>
);

export const ApplicationHydrateOperationState: React.FunctionComponent<Props> = ({hydrateOperationState, waitingForController, serverHydratorDisabledAtAPI}) => {
    if (waitingForController) {
        return (
            <div>
                <div className='white-box'>
                    <div className='white-box__details'>
                        <div className='row white-box__details-row'>
                            <div className='columns small-3'>STATUS</div>
                            <div className='columns small-9'>Waiting for application controller</div>
                        </div>
                        <div className='application-hydrate-operation-state__waiting-body columns small-12'>
                            <p>
                                This Application has <code>spec.sourceHydrator</code> set, but the API response does not yet include{' '}
                                <code>status.sourceHydrator.currentOperation</code>. The application controller normally writes that field when it reconciles hydration.
                            </p>
                            <p>
                                The Argo CD API server can report source hydrator as enabled in UI settings even when the application controller is still running with hydrator
                                disabled—each component has its own <code>ARGOCD_HYDRATOR_ENABLED</code> flag (for example via <code>hydrator.enabled</code> in the{' '}
                                <code>argocd-cmd-params-cm</code> ConfigMap).
                            </p>
                            <p>
                                If you set <code>hydrator.enabled</code> to <code>true</code> in <code>argocd-cmd-params-cm</code>, restart the{' '}
                                <strong>argocd-application-controller</strong> workload so it picks up the change, and ensure the server and controller are configured consistently.
                            </p>
                        </div>
                    </div>
                </div>
            </div>
        );
    }

    if (!hydrateOperationState) {
        if (serverHydratorDisabledAtAPI) {
            return (
                <div>
                    <div className='white-box'>
                        <div className='white-box__details'>
                            <ServerHydratorDisabledAtAPIExplanation hasHydrateOperationDetails={false} />
                        </div>
                    </div>
                </div>
            );
        }
        return null;
    }

    const operationAttributes = [
        {title: 'PHASE', value: hydrateOperationState.phase},
        ...(hydrateOperationState.message ? [{title: 'MESSAGE', value: hydrateOperationState.message}] : []),
        {title: 'STARTED AT', value: <Timestamp date={hydrateOperationState.startedAt} />},
        {
            title: 'DURATION',
            value: (
                <Ticker>
                    {time => (
                        <Duration
                            durationS={
                                ((hydrateOperationState.finishedAt && moment(hydrateOperationState.finishedAt)) || moment(time)).diff(moment(hydrateOperationState.startedAt)) /
                                1000
                            }
                        />
                    )}
                </Ticker>
            )
        }
    ];

    if (hydrateOperationState.finishedAt && hydrateOperationState.phase !== 'Hydrating') {
        operationAttributes.push({title: 'FINISHED AT', value: <Timestamp date={hydrateOperationState.finishedAt} />});
    }
    if (hydrateOperationState.drySHA) {
        operationAttributes.push({
            title: 'DRY REVISION',
            value: (
                <div>
                    <Revision repoUrl={hydrateOperationState.sourceHydrator.drySource.repoURL} revision={hydrateOperationState.drySHA} />
                </div>
            )
        });
    }
    if (hydrateOperationState.finishedAt && hydrateOperationState.hydratedSHA) {
        operationAttributes.push({
            title: 'HYDRATED REVISION',
            value: (
                <div>
                    <Revision repoUrl={hydrateOperationState.sourceHydrator.drySource.repoURL} revision={hydrateOperationState.hydratedSHA} />
                </div>
            )
        });
    }
    return (
        <div>
            {serverHydratorDisabledAtAPI && (
                <div className='white-box' style={{marginBottom: '12px'}}>
                    <div className='white-box__details'>
                        <ServerHydratorDisabledAtAPIExplanation hasHydrateOperationDetails={true} />
                    </div>
                </div>
            )}
            <div className='white-box'>
                <div className='white-box__details'>
                    {operationAttributes.map(attr => (
                        <div className='row white-box__details-row' key={attr.title}>
                            <div className='columns small-3'>{attr.title}</div>
                            <div className='columns small-9'>{attr.value}</div>
                        </div>
                    ))}
                </div>
            </div>
        </div>
    );
};

ApplicationHydrateOperationState.contextTypes = {
    apis: PropTypes.object
};
