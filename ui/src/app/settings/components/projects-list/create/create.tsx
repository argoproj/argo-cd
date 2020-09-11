import * as React from 'react';

export class ProjectCreate extends React.Component<null, {name: string; description: string}> {
    constructor() {
        super(null);
    }
    public render() {
        return (
            <div>
                New Project
                <input placeholder='Name' />
                <input placeholder='Description' />
            </div>
        );
    }
}
