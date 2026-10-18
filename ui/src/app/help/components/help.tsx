import * as React from 'react';
import {DataLoader, Page} from '../../shared/components';
import {Consumer} from '../../shared/context';
import {services} from '../../shared/services';
import {VersionMessage} from '../../shared/models';
import classNames from 'classnames';
import {CliDownload, cliDownloads, clientOS} from './cli-download';

require('./help.scss');

interface CliDownloadLinksProps {
    binaryUrls: Record<string, string>;
    hostArch: string;
    version: string;
}

const CliDownloadLinks = (props: CliDownloadLinksProps) => {
    // Best effort: the size of the binary served by this server is only known from its
    // Content-Length. When the request fails (auth, proxies) the label omits the size.
    const [serverBinarySize, setServerBinarySize] = React.useState(0);
    React.useEffect(() => {
        let alive = true;
        fetch('download/argocd-linux', {method: 'HEAD'})
            .then(response => (response.ok ? Number(response.headers.get('content-length')) : 0))
            .then(size => {
                if (alive) {
                    setServerBinarySize(size);
                }
            })
            .catch(() => {
                // Ignore: the size is informational only.
            });
        return () => {
            alive = false;
        };
    }, []);

    const downloads = cliDownloads({
        os: clientOS(navigator.userAgent, navigator.platform),
        hostArch: props.hostArch,
        version: props.version,
        serverBinarySize,
        binaryUrls: props.binaryUrls
    });

    return (
        <>
            {downloads.map((download: CliDownload) => (
                <React.Fragment key={download.key}>
                    <a href={download.href} className='user-info-panel-buttons argo-button argo-button--base'>
                        <i className={classNames('fab', download.icon)} />
                        {` ${download.label}`}
                    </a>
                    &nbsp;
                </React.Fragment>
            ))}
        </>
    );
};

export const Help = () => {
    return (
        <DataLoader
            load={async () => {
                // Return a Promise (not an Observable): DataLoader's observable branch has a
                // completion race that, under React's batched updates, clobbers the loaded value
                // with `undefined` when every source completes (as these promise-backed ones do).
                const [settings, version] = await Promise.all([
                    services.authService.settings(),
                    // Best-effort, informative only: the release link and the download hints are
                    // derived from the server's version and architecture.
                    services.version.version().catch(() => ({Platform: '', Version: ''}) as VersionMessage)
                ]);
                return {
                    binaryUrls: settings.help.binaryUrls || {},
                    // Platform is "<os>/<arch>" (e.g. "linux/amd64"); may be empty.
                    hostArch: (version.Platform || '').split('/')[1] || '',
                    version: version.Version || ''
                };
            }}>
            {(settings?: {binaryUrls: Record<string, string>; hostArch: string; version: string}) => {
                const binaryUrls = settings?.binaryUrls || {};
                const hostArch = settings?.hostArch || '';
                const version = settings?.version || '';
                return (
                    <Consumer>
                        {() => (
                            <Page title='Help'>
                                <div className='row'>
                                    <div className='columns large-4 small-6'>
                                        <div className='help-box'>
                                            <p>New to Argo CD?</p>
                                            <a className='user-info-panel-buttons argo-button argo-button--base' href='https://argo-cd.readthedocs.io'>
                                                Read the docs
                                            </a>
                                        </div>
                                    </div>
                                    <div className='columns large-4 small-6'>
                                        <div className='help-box'>
                                            <p>Want to download the CLI tool?</p>
                                            {/* The server only publishes the Linux binary it was built for, so the
                                                platform-neutral GitHub release page is offered next to it. */}
                                            <CliDownloadLinks binaryUrls={binaryUrls} hostArch={hostArch} version={version} />
                                        </div>
                                    </div>
                                    <div className='columns large-4 small-6'>
                                        <div className='help-box'>
                                            <p>You want to develop against Argo CD's API?</p>
                                            <a className='user-info-panel-buttons argo-button argo-button--base' href='swagger-ui'>
                                                Open the API docs
                                            </a>
                                        </div>
                                    </div>
                                </div>
                            </Page>
                        )}
                    </Consumer>
                );
            }}
        </DataLoader>
    );
};
