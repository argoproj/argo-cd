import {DataLoader, Tooltip} from 'argo-ui';
import * as React from 'react';
import {Timestamp} from '../../../shared/components/timestamp';
import {RevisionMetadata} from '../../../shared/models';
import {services} from '../../../shared/services';

export const RevisionMetadataPanel = (props: {appName: string; type: string; revision: string}) => {
    if (props.type === 'helm') {
        return <React.Fragment />;
    }
    return (
        <DataLoader input={props} load={input => services.applications.revisionMetadata(input.appName, props.revision)}>
            {(m: RevisionMetadata) => (
                <Tooltip
                    content={
                        <span>
                            {m.author && <React.Fragment>Authored by {m.author}</React.Fragment>}
                            <br />
                            {m.date && <Timestamp date={m.date} />}
                            <br />
                            {m.tags && (
                                <span>
                                    Tags: {m.tags}
                                    <br />
                                </span>
                            )}
                            {m.signatureInfo}
                            <br />
                            {m.message}
                        </span>
                    }
                    placement='bottom'
                    allowHTML={true}>
                    <div className='application-status-panel__item-name'>
                        {m.author && (
                            <React.Fragment>
                                Authored by {m.author} - {m.signatureInfo}
                                <br />
                            </React.Fragment>
                        )}
                        {m.tags && (
                            <span>
                                Tagged {m.tags.join(', ')}
                                <br />
                            </span>
                        )}
                        {m.message}
                    </div>
                </Tooltip>
            )}
        </DataLoader>
    );
};
