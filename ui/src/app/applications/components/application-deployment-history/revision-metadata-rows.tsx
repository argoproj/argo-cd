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

export class RevisionMetadataRows extends React.Component<Props, State> {
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
            return (
                <div>
                    <div className='row'>
                        <div className='columns small-3'>Authored by</div>
                        <div className='columns small-9'>
                            {author}<br/>
                            <Timestamp date={date}/>
                        </div>
                    </div>
                    {tags && (
                        <div className='row'>
                            <div className='columns small-3'>Tagged</div>
                            <div className='columns small-9'>{tags.join(', ')}</div>
                        </div>
                    )}
                    <div className='row'>
                        <div className='columns small-3'/>
                        <div className='columns small-9'>{message}</div>
                    </div>
                </div>
            );
        }
        if (this.state.error) {
            return <div>{this.state.error.message}</div>;
        }
        return <div>Loading...</div>;
    }
}
