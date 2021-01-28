import {DataLoader, SlidingPanel, Tooltip} from 'argo-ui';
import * as React from 'react';
import {VersionMessage} from '../../models';

interface VersionPanelProps {
    isShown: boolean;
    onClose: () => void;
    version: Promise<VersionMessage>;
}

type CopyState = 'success' | 'failed' | undefined;

export class VersionPanel extends React.Component<VersionPanelProps, {copyState: CopyState}> {
    private readonly header = 'Argo CD Server Version';

    constructor(props: VersionPanelProps) {
        super(props);
        this.state = {copyState: undefined};
    }

    public render() {
        return (
            <DataLoader load={() => this.props.version}>
                {version => {
                    return (
                        <SlidingPanel header={this.header} isShown={this.props.isShown} onClose={() => this.props.onClose()} hasCloseButton={true} isNarrow={true}>
                            <div className='argo-table-list'>{this.buildVersionTable(version)}</div>
                            <div>
                                <Tooltip content='Copy all version info as JSON'>{this.getCopyButton(version)}</Tooltip>
                            </div>
                        </SlidingPanel>
                    );
                }}
            </DataLoader>
        );
    }

    /**
     * Formats the version data and renders the table rows.
     */
    private buildVersionTable(version: VersionMessage): JSX.Element {
        const formattedVersion = {
            'Argo CD': version.Version,
            'Build Date': version.BuildDate,
            'Go': version.GoVersion,
            'Compiler': version.Compiler,
            'Platform': version.Platform,
            'ksonnet': version.KsonnetVersion,
            'jsonnet': version.JsonnetVersion,
            'kustomize': version.KustomizeVersion,
            'Helm': version.HelmVersion,
            'kubectl': version.KubectlVersion
        };

        return (
            <React.Fragment>
                {Object.entries(formattedVersion).map(([key, value]) => {
                    return (
                        <div className='argo-table-list__row' key={key}>
                            <div className='row'>
                                <div className='columns small-4' title={key}>
                                    <strong>{key}</strong>
                                </div>
                                <div className='columns'>
                                    {value && (
                                        <Tooltip content={value}>
                                            <span>{value}</span>
                                        </Tooltip>
                                    )}
                                </div>
                            </div>
                        </div>
                    );
                })}
            </React.Fragment>
        );
    }

    private getCopyButton(version: VersionMessage): JSX.Element {
        let img: string;
        let text: string;
        if (this.state.copyState === 'success') {
            img = 'fa-check';
            text = 'Copied';
        } else if (this.state.copyState === 'failed') {
            img = 'fa-times';
            text = 'Copy Failed';
        } else {
            img = 'fa-copy';
            text = 'Copy JSON';
        }

        return (
            <button className='argo-button argo-button--base' style={{marginTop: '1em', minWidth: '18ch'}} onClick={() => this.onCopy(version)}>
                <i className={'fa ' + img} />
                &nbsp;&nbsp;{text}
            </button>
        );
    }

    private async onCopy(version: VersionMessage): Promise<void> {
        const stringifiedVersion = JSON.stringify(version, undefined, 4);
        try {
            await navigator.clipboard.writeText(stringifiedVersion);
            this.setState({copyState: 'success'});
        } catch (err) {
            this.setState({copyState: 'failed'});
        }

        setTimeout(() => {
            this.setState({copyState: undefined});
        }, 750);
    }
}
