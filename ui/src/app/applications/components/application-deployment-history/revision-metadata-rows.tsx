import {DataLoader} from 'argo-ui';
import * as React from 'react';
import {Timestamp} from '../../../shared/components/timestamp';
import {ApplicationSource, RevisionMetadata} from '../../../shared/models';
import {services} from '../../../shared/services';

export const RevisionMetadataRows = (props: {applicationName: string; source: ApplicationSource}) => {
    if (props.source.chart) {
        return (
            <div>
                <div className='row'>
                    <div className='columns small-3'>Helm Chart </div>
                    <div className='columns small-9'>{props.source.chart}</div>
                </div>
                <div className='row'>
                    <div className='columns small-3'>Version</div>
                    <div className='columns small-9'>v{props.source.targetRevision}</div>
                </div>
            </div>
        );
    }
    return (
        <DataLoader input={props} load={input => services.applications.revisionMetadata(input.applicationName, input.source.targetRevision)}>
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
                    {m.tags && (
                        <div className='row'>
                            <div className='columns small-3'>Tagged</div>
                            <div className='columns small-9'>{m.tags.join(', ')}</div>
                        </div>
                    )}
                    {m.message && (
                        <div className='row'>
                            <div className='columns small-3' />
                            <div className='columns small-9'>{m.message}</div>
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
