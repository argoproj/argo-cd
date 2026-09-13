import * as models from '../../../shared/models';

function hasOwnProperty(value: object | null | undefined, field: PropertyKey): boolean {
    return value != null && Object.prototype.hasOwnProperty.call(value, field);
}

export function validateApplicationCreate(app: models.Application, multiSourceMode: boolean): Record<string, string | undefined> {
    const hasHydrator = !!app.spec.sourceHydrator;
    const source = app.spec.source;

    const destinationErrors = {
        'spec.destination.server':
            !app.spec.destination.server && (!hasOwnProperty(app.spec.destination, 'name') || app.spec.destination.name === '') ? 'Cluster URL is required' : undefined,
        'spec.destination.name':
            !app.spec.destination.name && (!hasOwnProperty(app.spec.destination, 'server') || app.spec.destination.server === '') ? 'Cluster name is required' : undefined
    };

    if (multiSourceMode && !hasHydrator) {
        const errors: Record<string, string | undefined> = {
            'metadata.name': !app.metadata.name ? 'Application Name is required' : undefined,
            'spec.project': !app.spec.project ? 'Project Name is required' : undefined,
            ...destinationErrors
        };
        const sources = app.spec.sources || [];
        for (let i = 0; i < sources.length; i++) {
            const currentSource = sources[i];
            errors[`spec.sources[${i}].repoURL`] = !currentSource?.repoURL ? 'Repository URL is required' : undefined;
            errors[`spec.sources[${i}].targetRevision`] = !currentSource?.targetRevision && hasOwnProperty(currentSource, 'chart') ? 'Version is required' : undefined;
            errors[`spec.sources[${i}].path`] = !currentSource?.path && !currentSource?.chart && !currentSource?.ref ? 'Path or Ref is required' : undefined;
            errors[`spec.sources[${i}].chart`] = !currentSource?.path && !currentSource?.chart && !currentSource?.ref ? 'Chart is required' : undefined;
        }
        return errors;
    }

    return {
        'metadata.name': !app.metadata.name ? 'Application Name is required' : undefined,
        'spec.project': !app.spec.project ? 'Project Name is required' : undefined,
        'spec.source.repoURL': !hasHydrator && !source?.repoURL ? 'Repository URL is required' : undefined,
        'spec.source.targetRevision': !hasHydrator && !source?.targetRevision && hasOwnProperty(source, 'chart') ? 'Version is required' : undefined,
        'spec.source.path': !hasHydrator && !source?.path && !source?.chart ? 'Path is required' : undefined,
        'spec.source.chart': !hasHydrator && !source?.path && !source?.chart ? 'Chart is required' : undefined,
        ...destinationErrors
    };
}
