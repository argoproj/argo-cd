import * as React from 'react';

export class ErrorBoundary extends React.Component<{message?: string}, {hasError: boolean}> {
    constructor(props: any) {
        super(props);
        this.state = {hasError: false};
    }

    static getDerivedStateFromError() {
        return {hasError: true};
    }

    render() {
        if (this.state.hasError) {
            return <h1>{this.props.message ? this.props.message : 'Something went wrong.'}</h1>;
        }

        return this.props.children;
    }
}
