import {DataLoader, Tooltip} from 'argo-ui';
import * as React from 'react';
import {Timestamp} from '../../../shared/components/timestamp';
import {ApplicationSource, RevisionMetadata} from '../../../shared/models';
import {services} from '../../../shared/services';

export const RevisionMetadataPanel = (props: {
    applicationName: string;
    source: ApplicationSource;
}) => {
    if (props.source.chart) {
        return (
            <div className='application-status-panel__item-name'>
                Helm Chart {props.source.chart} v{props.source.targetRevision}
            </div>
        );
    }
    return (
        <DataLoader input={props}
            load={(input) => services.applications.revisionMetadata(input.applicationName, props.source.targetRevision || '')}
        >{(m: RevisionMetadata) => (
            <Tooltip content={(
                <span>
                    {m.author && <React.Fragment>Authored by {m.author}</React.Fragment>}
                    {m.date && <Timestamp date={m.date}/>}<br/>
                    {m.tags && (<span>Tags: {m.tags}<br/></span>)}
                    (m.message}
                </span>
            )} placement='bottom' allowHTML={true}>
                <div className='application-status-panel__item-name'>
                    {m.author && <React.Fragment>Authored by {m.author}<br/></React.Fragment>}
                    {m.tags && <span>Tagged {m.tags.join(', ')}<br/></span>}
                    {m.message}
                </div>
            </Tooltip>
        )}</DataLoader>
    );
};
