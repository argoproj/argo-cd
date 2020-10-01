import {DataLoader, SlidingPanel, Tooltip} from 'argo-ui';
import * as React from 'react';
import {VersionMessage} from '../../shared/models';

import './version-info.scss';

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
                            <div className='version-info-table argo-table-list'>{this.buildVersionTable(version)}</div>
                            <div className='version-copy-btn-container'>
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
        // match the order/format of `argocd version`
        // but leave out 'version' from the titles; that's implied by us being in the version info panel.
        // These key/values are rendered to the user as written in this object
        const formattedVersion = {
            'Argo CD': version.Version,
            'Build Date': version.BuildDate,
            'Go': version.GoVersion,
            'Compiler': version.Compiler,
            'Platform': version.Platform,
            'ksonnet': version.KsonnetVersion,
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
                                    <Tooltip content={value}>
                                        <span>{value}</span>
                                    </Tooltip>
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
            <button className='argo-button argo-button--base' onClick={() => this.onCopy(version)}>
                <i className={'fa ' + img} />
                {text}
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
