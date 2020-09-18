import {DataLoader, SlidingPanel, Tooltip} from 'argo-ui';
import * as React from 'react';
import * as CopyToClipboard from 'react-copy-to-clipboard';
import {VersionMessage} from '../../shared/models';

import './version-info.scss';

interface VersionPanelProps {
    isShown: boolean;
    onClose: () => void;
    version: Promise<VersionMessage>;
}

export class VersionPanel extends React.Component<VersionPanelProps, {justCopied: boolean}> {
    private readonly header = 'Argo CD Server Version';

    constructor(props: VersionPanelProps) {
        super(props);
        this.state = {justCopied: false};
    }

    public render() {
        return (
            <DataLoader load={() => this.props.version}>
                {version => {
                    return (
                        <SlidingPanel header={this.header} isShown={this.props.isShown} onClose={() => this.props.onClose()} hasCloseButton={true} isNarrow={true}>
                            <div className='version-info-table argo-table-list'>
                                {/* <div className='argo-table-list__head'>
                                    <div className='row'>
                                        <div className='columns small-4'>Tool</div>
                                        <div className='columns small-4'>Version</div>
                                    </div>
                                </div> */}
                                {this.buildVersionTable(version)}
                            </div>
                            <div className='version-copy-btn-container'>
                                <Tooltip content='Copy all version info as JSON'>
                                    <CopyToClipboard text={JSON.stringify(version, undefined, 4)} onCopy={() => this.onCopy()}>
                                        <button className='argo-button argo-button--base'>
                                            <i className={'fa ' + (this.state.justCopied ? 'fa-check' : 'fa-copy')} />
                                            Copy JSON
                                        </button>
                                    </CopyToClipboard>
                                </Tooltip>
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
            // 'Git Commit':
            // 'Git Tree State':
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

    private onCopy(): void {
        this.setState({justCopied: true});
        setTimeout(() => {
            this.setState({justCopied: false});
        }, 500);
    }
}
