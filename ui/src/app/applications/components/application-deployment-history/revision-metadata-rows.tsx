import {DataLoader} from 'argo-ui';
import * as React from 'react';
import {Timestamp} from '../../../shared/components/timestamp';
import {ApplicationSource, RevisionMetadata, ChartDetails} from '../../../shared/models';
import {services} from '../../../shared/services';

export const RevisionMetadataRows = (props: {applicationName: string; applicationNamespace: string; source: ApplicationSource; index: number; versionId: number}) => {
    if (props.source.chart) {
        return (
            <DataLoader
                input={props}
                load={input =>
                    services.applications.revisionChartDetails(input.applicationName, input.applicationNamespace, input.source.targetRevision, input.index, input.versionId)
                }>
                {(m: ChartDetails) => (
                    <div>
                        <div className='row'>
                            <div className='columns small-3'>Helm Chart:</div>
                            <div className='columns small-9'>
                                {props.source.chart}&nbsp;
                                {m.home && (
                                    <a
                                        title={m.home}
                                        onClick={e => {
                                            e.stopPropagation();
                                            window.open(m.home);
                                        }}>
                                        <i className='fa fa-external-link-alt' />
                                    </a>
                                )}
                            </div>
                        </div>
                        {m.description && (
                            <div className='row'>
                                <div className='columns small-3'>Description:</div>
                                <div className='columns small-9'>{m.description}</div>
                            </div>
                        )}
                        {m.maintainers && m.maintainers.length > 0 && (
                            <div className='row'>
                                <div className='columns small-3'>Maintainers:</div>
                                <div className='columns small-9'>{m.maintainers.join(', ')}</div>
                            </div>
                        )}
                    </div>
                )}
            </DataLoader>
        );
    }
    return (
        <DataLoader
            input={props}
            load={input => services.applications.revisionMetadata(input.applicationName, input.applicationNamespace, input.source.targetRevision, input.index, input.versionId)}>
            {(m: RevisionMetadata) => (
                <div>
                    <div className='row'>
                        <div className='columns small-3'>Authored by</div>
                        <div className='columns small-9'>
                            {m.author || 'unknown'}
                            <br />
                            {m.date && <Timestamp date={m.date} />}
                        </div>
                    </div>
                    {m.message && (
                        <div className='row'>
                            <div className='columns small-3' />
                            <div className='columns small-9'>{m.message?.split('\n')[0].slice(0, 64)}</div>
                        </div>
                    )}
                    <div className='row'>
                        <div className='columns small-3'>GPG signature</div>
                        <div className='columns small-9'>{m.signatureInfo || '-'}</div>
                    </div>
                </div>
            )}
        </DataLoader>
    );
};
