import {DataLoader, SlidingPanel, Tooltip} from 'argo-ui';
import React, {useState} from 'react';
import {VersionMessage} from '../../models';
import {services} from '../../services';
import {ThemeWrapper} from '../layout/layout';

interface VersionPanelProps {
    isShown: boolean;
    onClose: () => void;
    version: Promise<VersionMessage>;
}

type CopyState = 'success' | 'failed' | undefined;

export function VersionPanel({isShown, onClose, version}: VersionPanelProps) {
    const [copyState, setCopyState] = useState<CopyState>(undefined);
    const header = 'Argo CD Server Version';

    const buildVersionTable = (version: VersionMessage): JSX.Element => {
        const formattedVersion = {
            'Argo CD': version.Version,
            'Build Date': version.BuildDate,
            'Go Version': version.GoVersion,
            'Go Compiler': version.Compiler,
            'Platform': version.Platform,
            'jsonnet': version.JsonnetVersion,
            'kustomize': version.KustomizeVersion,
            'Helm': version.HelmVersion,
            'kubectl': version.KubectlVersion
        };

        return (
            <>
                {Object.entries(formattedVersion).map(([key, value]) => (
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
                ))}
            </>
        );
    };

    const getCopyButton = (version: VersionMessage): JSX.Element => {
        let img: string;
        let text: string;
        if (copyState === 'success') {
            img = 'fa-check';
            text = 'Copied';
        } else if (copyState === 'failed') {
            img = 'fa-times';
            text = 'Copy Failed';
        } else {
            img = 'fa-copy';
            text = 'Copy JSON';
        }

        return (
            <button className='argo-button argo-button--base' style={{marginTop: '1em', minWidth: '18ch'}} onClick={() => onCopy(version)}>
                <i className={'fa ' + img} />
                &nbsp;&nbsp;{text}
            </button>
        );
    };

    const onCopy = async (version: VersionMessage): Promise<void> => {
        const stringifiedVersion = JSON.stringify(version, undefined, 4) + '\n';
        try {
            await navigator.clipboard.writeText(stringifiedVersion);
            setCopyState('success');
        } catch (err) {
            setCopyState('failed');
        }
        setTimeout(() => {
            setCopyState(undefined);
        }, 750);
    };

    return (
        <DataLoader load={() => services.viewPreferences.getPreferences()}>
            {pref => (
                <DataLoader load={() => version}>
                    {version => (
                        <ThemeWrapper theme={pref.theme}>
                            <SlidingPanel header={header} isShown={isShown} onClose={onClose} hasCloseButton={true} isNarrow={true}>
                                <div className='argo-table-list'>{buildVersionTable(version)}</div>
                                <div>
                                    <Tooltip content='Copy all version info as JSON'>{getCopyButton(version)}</Tooltip>
                                </div>
                            </SlidingPanel>
                        </ThemeWrapper>
                    )}
                </DataLoader>
            )}
        </DataLoader>
    );
}
