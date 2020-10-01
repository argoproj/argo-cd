import {DataLoader, Tooltip} from 'argo-ui';
import * as React from 'react';
import {VersionMessage} from '../../shared/models';

import './version-info.scss';

interface VersionButtonProps {
    onClick: () => void;
    version: Promise<VersionMessage>;
}

export class VersionButton extends React.Component<VersionButtonProps> {
    constructor(props: VersionButtonProps) {
        super(props);
    }

    public render() {
        return (
            <DataLoader load={() => this.props.version}>
                {version => (
                    <React.Fragment>
                        <Tooltip content={version.Version}>
                            <span className='version-info-btn' onClick={this.props.onClick}>
                                {version.Version}
                            </span>
                        </Tooltip>
                    </React.Fragment>
                )}
            </DataLoader>
        );
    }
}
