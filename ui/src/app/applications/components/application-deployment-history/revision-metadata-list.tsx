import * as React from 'react';
import * as models from '../../../shared/models';
import {services} from "../../../shared/services";

interface Props {
    applicationName: string;
    revision: string;
}

interface State {
    revisionMetadata?: models.RevisionMetadata;
    error?: Error;
}

export class RevisionMetadataList extends React.Component<Props, State> {
    constructor(props: Props) {
        super(props);
        this.state = {};
    }

    componentDidMount() {
        services.applications.revisionMetadata(this.props.applicationName, this.props.revision)
            .then(value => this.setState({revisionMetadata: value}))
            .catch(e => this.setState({error: e}))
    }

    render() {
        return this.state.revisionMetadata ? (
            <div>
                <div className='row'>
                    <div className='columns small-2'>AUTHOR:</div>
                    <div className='columns small-10'>{this.state.revisionMetadata.author}</div>
                </div>
                {this.state.revisionMetadata.tags && (
                    <div className='row'>
                        <div className='columns small-2'>TAGS:</div>
                        <div
                            className='columns small-10'>{this.state.revisionMetadata.tags.join(', ') || "∅"}</div>
                    </div>
                )}
                <div className='row'>
                    <div className='columns small-12'>{this.state.revisionMetadata.message}</div>
                </div>
            </div>
        ) : this.state.error ? (
            <div>{this.state.error.message}</div>
        ) : <div>Loading...</div>;
    }
}
