import * as React from 'react';
import {Project} from '../../../shared/models';
import {ProjectParams, services} from '../../../shared/services';

interface ProjectCreateProps {
    cancel?: () => void;
    callback?: (proj: Project) => void;
}

interface ProjectCreateState {
    name: string;
    description: string;
    error: string;
}

export class ProjectCreate extends React.Component<ProjectCreateProps, ProjectCreateState> {
    public render() {
        return (
            <React.Fragment>
                <div style={{marginTop: '1.5em'}}>
                    <input className='argo-field' type='text' placeholder='Name' onChange={e => this.setState({name: e.target.value})} />
                    <input className='argo-field' type='text' placeholder='Description' onChange={e => this.setState({description: e.target.value})} />
                </div>
                {this.state && this.state.error && <div style={{marginTop: '1.5em'}}>Error: {this.state.error.toString()}</div>}
                <div style={{marginTop: '1.5em'}}>
                    <button
                        onClick={() => {
                            services.projects
                                .create({description: this.state.description, name: this.state.name} as ProjectParams)
                                .then(res => {
                                    this.props.callback(res);
                                })
                                .catch(err => {
                                    this.setState({error: JSON.parse(err.response.text).message});
                                });
                        }}
                        className='argo-button argo-button--base'>
                        Create
                    </button>
                    {this.props.cancel && (
                        <button onClick={this.props.cancel} className='argo-button argo-button--base-o'>
                            Cancel
                        </button>
                    )}
                </div>
            </React.Fragment>
        );
    }
}
