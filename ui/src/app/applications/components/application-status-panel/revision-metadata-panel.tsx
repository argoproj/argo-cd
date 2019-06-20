import {Tooltip} from 'argo-ui';
import * as React from 'react';
import {Timestamp} from '../../../shared/components/timestamp';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';

interface Props {
    applicationName: string;
    revision: string;
}

interface State {
    revisionMetadata?: models.RevisionMetadata;
    error?: Error;
}

export class RevisionMetadataPanel extends React.Component<Props, State> {
    constructor(props: Props) {
        super(props);
        this.state = {};
    }

    public componentDidMount() {
        services.applications.revisionMetadata(this.props.applicationName, this.props.revision)
            .then((value) => this.setState({revisionMetadata: value}))
            .catch((e) => this.setState({error: e}));
    }

    public render() {
        const revisionMetadata = this.state.revisionMetadata;
        if (revisionMetadata) {
            const author = revisionMetadata.author;
            const date = revisionMetadata.date;
            const tags = revisionMetadata.tags;
            const message = revisionMetadata.message;
            const tip = (
                <span>
                    <span>Authored by {author} <Timestamp date={date}/></span><br/>
                    {tags && (<span>Tags: {tags}<br/></span>)}
                    <span>{message}</span>
                </span>
            );

            return (
                <Tooltip content={tip} placement='bottom' allowHTML={true}>
                    <div className='application-status-panel__item-name'>
                        Authored by {author}<br/>
                        {tags && <span>Tagged {tags.join(', ')}<br/></span>}
                        {message}
                        </div>
                </Tooltip>
            );
        }
        if (this.state.error) {
            return <div className='application-status-panel__item-name'>{this.state.error.message}</div>;
        }
        return <div className='application-status-panel__item-name'>Loading....</div>;

    }
}
