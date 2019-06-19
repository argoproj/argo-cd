import * as React from 'react';
import * as models from '../../../shared/models';
import {services} from "../../../shared/services";
import {Tooltip} from 'argo-ui';

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

    componentDidMount() {
        services.applications.revisionMetadata(this.props.applicationName, this.props.revision)
            .then(value => this.setState({revisionMetadata: value}))
            .catch(e => this.setState({error: e}))
    }

    render() {
        if (this.state.revisionMetadata) {
            const author = this.state.revisionMetadata.author;
            const tags = this.state.revisionMetadata.tags ? this.state.revisionMetadata.tags.join(", ") : "∅";
            const message = this.state.revisionMetadata.message;
            const tip = `Author: ${author}<br/>Tags: ${tags}<br/>${message}`;

            return <Tooltip content={tip} placement="bottom" allowHTML={true}>
                <div>
                    <div className='application-status-panel__item-name'>{author}</div>
                    <div className='application-status-panel__item-name'>Tags: {tags}</div>
                    <div className='application-status-panel__item-name'>{message}</div>
                </div>
            </Tooltip>;
        }
        if (this.state.error) {
            return <div className='application-status-panel__item-name'>{this.state.error.message}</div>;
        }
        return <div className='application-status-panel__item-name'>Loading....</div>;

    }
}
