// Read-only fixture API for exercising the real Argo CD UI without Kubernetes.
/* global require, __dirname */
const http = require('node:http');
const process = require('node:process');
const path = require('node:path');
const {spawn} = require('node:child_process');

const uiRoot = path.resolve(__dirname, '..');
const apiPort = Number(process.env.DEMO_API_PORT || 18080);
const uiPort = Number(process.env.DEMO_UI_PORT || 4000);
const cluster = 'https://kubernetes.default.svc';
const repo = 'https://github.com/argoproj/argocd-example-apps.git';
const fixtures = [
    ['demo-outofsync', 'OutOfSync', 'Healthy', 'production', 'default'],
    ['demo-degraded', 'Synced', 'Degraded', 'production', 'default'],
    ['demo-both', 'OutOfSync', 'Degraded', 'production', 'default'],
    ['demo-healthy', 'Synced', 'Healthy', 'production', 'default'],
    ['demo-progressing', 'Synced', 'Progressing', 'staging', 'sandbox'],
    ['demo-staging-degraded', 'Synced', 'Degraded', 'staging', 'sandbox']
].map(([name, sync, health, namespace, project]) => ({
    apiVersion: 'argoproj.io/v1alpha1',
    kind: 'Application',
    metadata: {name, namespace: 'argocd', uid: name, resourceVersion: '1', creationTimestamp: '2026-11-20T10:00:00Z', labels: {environment: namespace, team: project}},
    spec: {project, source: {repoURL: repo, path: 'guestbook', targetRevision: 'HEAD'}, destination: {server: cluster, namespace}},
    status: {sync: {status: sync, revision: 'demo'}, health: {status: health}, summary: {}, resources: [], reconciledAt: '2026-11-20T10:00:00Z'}
}));

const settings = {
    url: `http://127.0.0.1:${uiPort}`,
    statusBadgeEnabled: false,
    userLoginsDisabled: true,
    controllerNamespace: 'argocd',
    uiBannerContent: 'LOCAL FILTER DEMO · Sample applications · Read only · No Kubernetes cluster connected',
    uiBannerPermanent: true,
    uiBannerPosition: 'top',
    resourceOverrides: {},
    help: {binaryUrls: {}},
    kustomizeVersions: []
};

const server = http.createServer((request, response) => {
    const url = new URL(request.url, `http://127.0.0.1:${apiPort}`);
    const json = (value, status = 200) => {
        response.writeHead(status, {'Content-Type': 'application/json', 'Cache-Control': 'no-store'});
        response.end(JSON.stringify(value));
    };
    if (request.method !== 'GET') {
        return json({message: 'This demo is read only.'}, 405);
    }
    if (url.pathname === '/healthz') return json({ok: true, mode: 'fixtures'});
    if (url.pathname === '/api/version') return json({Version: 'v3.7.0+filter-demo', BuildDate: '2026-11-20', GitCommit: 'local', GoVersion: 'demo'});
    if (url.pathname === '/api/v1/settings') return json(settings);
    if (url.pathname === '/api/v1/session/userinfo') return json({loggedIn: true, username: 'filter-demo', iss: 'argocd', groups: []});
    if (url.pathname.startsWith('/api/v1/account/can-i/')) return json({value: url.pathname.includes('/get/') ? 'yes' : 'no'});
    if (url.pathname === '/api/v1/clusters') {
        return json({items: [{name: 'in-cluster', server: cluster, connectionState: {status: 'Successful'}, info: {connectionState: {status: 'Successful'}}}]});
    }
    if (url.pathname === '/api/v1/projects') {
        return json({items: ['default', 'sandbox'].map(name => ({metadata: {name}, spec: {destinations: [{server: '*', namespace: '*'}], sourceRepos: ['*']}}))});
    }
    if (url.pathname === '/api/v1/applications') {
        const projects = url.searchParams.getAll('projects');
        const names = url.searchParams.getAll('names');
        const namespace = url.searchParams.get('appNamespace');
        return json({
            metadata: {resourceVersion: '1'},
            items: fixtures.filter(
                app =>
                    (!projects.length || projects.includes(app.spec.project)) &&
                    (!names.length || names.includes(app.metadata.name) || names.includes(`argocd/${app.metadata.name}`)) &&
                    (!namespace || namespace === app.metadata.namespace)
            )
        });
    }
    if (url.pathname === '/api/v1/stream/applications') {
        response.writeHead(200, {'Content-Type': 'text/event-stream', 'Cache-Control': 'no-cache', 'Connection': 'keep-alive'});
        response.write(': fixture stream\n\n');
        const heartbeat = setInterval(() => response.write(': heartbeat\n\n'), 15000);
        response.on('close', () => clearInterval(heartbeat));
        return;
    }
    if (url.pathname === '/api/v1/settings/plugins') return json({plugins: []});
    if (url.pathname === '/api/v1/applicationsets') return json({items: []});
    return json({message: 'This endpoint is not part of the filter demo.'}, 404);
});

let ui;
server.on('error', error => {
    console.error(error.message);
    process.exitCode = 1;
});
server.listen(apiPort, '127.0.0.1', () => {
    console.log(`Fixture API http://127.0.0.1:${apiPort}`);
    ui = spawn(
        process.execPath,
        [
            path.join(uiRoot, 'node_modules/webpack-dev-server/bin/webpack-dev-server.js'),
            '--config',
            './src/app/webpack.config.js',
            '--mode',
            'development',
            '--host',
            '127.0.0.1',
            '--port',
            String(uiPort)
        ],
        {
            cwd: uiRoot,
            stdio: 'inherit',
            env: {...process.env, ARGOCD_API_URL: `http://127.0.0.1:${apiPort}`, ARGO_VERSION: 'v3.7.0+filter-demo', REACT_COMPILER: '0'}
        }
    );
    ui.on('error', error => {
        console.error(error.message);
        server.close();
        process.exitCode = 1;
    });
    ui.on('exit', code => {
        server.close();
        server.closeAllConnections();
        process.exitCode = code || 0;
    });
    console.log(`Open http://127.0.0.1:${uiPort}/applications?q=sync%20%3D%20OutOfSync%20OR%20health%20%3D%20Degraded`);
});
for (const signal of ['SIGINT', 'SIGTERM']) {
    process.on(signal, () => {
        ui?.kill();
        server.close();
        server.closeAllConnections();
    });
}
