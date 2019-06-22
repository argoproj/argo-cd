import {DataLoader} from 'argo-ui';
import * as React from 'react';
import {Timestamp} from '../../../shared/components/timestamp';
import {RevisionMetadata} from '../../../shared/models';
import {services} from '../../../shared/services';

export const RevisionMetadataRows = (props: {
    applicationName: string;
    revision: string;
}) => {
    return (
        <DataLoader input={props}
                    load={(input) => services.applications.revisionMetadata(input.applicationName, input.revision || 'HEAD')}
        >{(m: RevisionMetadata) => (
            <div>
                <div className='row'>
                    <div className='columns small-3'>Authored by</div>
                    <div className='columns small-9'>
                        {m.author}<br/>
                        <Timestamp date={m.date}/>
                    </div>
                </div>
                {m.tags && (
                    <div className='row'>
                        <div className='columns small-3'>Tagged</div>
                        <div className='columns small-9'>{m.tags.join(', ')}</div>
                    </div>
                )}
                <div className='row'>
                    <div className='columns small-3'/>
                    <div className='columns small-9'>{m.message}</div>
                </div>
            </div>
        )}</DataLoader>
    );
};
