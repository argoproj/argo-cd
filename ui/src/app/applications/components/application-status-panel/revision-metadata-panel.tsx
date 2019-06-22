import {DataLoader, Tooltip} from 'argo-ui';
import * as React from 'react';
import {Timestamp} from '../../../shared/components/timestamp';
import {RevisionMetadata} from '../../../shared/models';
import {services} from '../../../shared/services';

export const RevisionMetadataPanel = (props: {
    applicationName: string;
    revision: string;
}) => {
    return (
        <DataLoader input={props}
                    load={(input) => services.applications.revisionMetadata(input.applicationName, input.revision || 'HEAD')}
        >{(m: RevisionMetadata) => (
            <Tooltip content={(
                <span>
            <span>Authored by {m.author} <Timestamp date={m.date}/></span><br/>
                    {m.tags && (<span>Tags: {m.tags}<br/></span>)}
                    <span>{m.message}</span>
        </span>
            )} placement='bottom' allowHTML={true}>
                <div className='application-status-panel__item-name'>
                    Authored by {m.author}<br/>
                    {m.tags && <span>Tagged {m.tags.join(', ')}<br/></span>}
                    {m.message}
                </div>
            </Tooltip>
        )}</DataLoader>
    );
};
