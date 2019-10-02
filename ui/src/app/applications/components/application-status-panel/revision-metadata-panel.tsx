import {DataLoader} from 'argo-ui';
import * as React from 'react';
import Moment from 'react-moment';
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
            <div className='application-status-panel__item-name'>
                {m.author && <React.Fragment>Authored by {m.author} </React.Fragment>}
                {m.date && <React.Fragment><Moment fromNow={true}>{m.date}</Moment><br/></React.Fragment>}
                {m.tags && <React.Fragment>Tagged {m.tags.join(', ')}<br/></React.Fragment>}
                {m.message}
            </div>
        )}</DataLoader>
    );
};
