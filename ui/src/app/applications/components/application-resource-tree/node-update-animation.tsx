import * as React from 'react';

export class NodeUpdateAnimation extends React.PureComponent<{resourceVersion: string}, {ready: boolean}> {
    constructor(props: {resourceVersion: string}) {
        super(props);
        this.state = {ready: false};
    }

    public render() {
        return this.state.ready && <div key={this.props.resourceVersion} className='application-resource-tree__node-animation' />;
    }

    public componentDidUpdate(prevProps: {resourceVersion: string}) {
        if (prevProps.resourceVersion && this.props.resourceVersion !== prevProps.resourceVersion) {
            this.setState({ready: true});
        }
    }
}
