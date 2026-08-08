import * as React from 'react';

import {MonacoEditor} from '../../../shared/components';
import * as models from '../../../shared/models';
import './application-resource-health-check.scss';

const SOURCE_LABELS: {[key: string]: string} = {
    'custom': 'Custom (defined in argocd-cm)',
    'built-in': 'Built-in (bundled with Argo CD)'
};

// Scripts at or under this length fit comfortably in a box sized exactly to their content, so the
// editor's internal scrollbar (and Monaco's "scroll past the last line" padding that comes with it)
// stays off - avoiding a tall, mostly-empty scroll area for what's usually a handful of lines.
// Longer scripts turn the scrollbar on and are capped to a fixed height instead, so they stay
// fully scrollable rather than growing to dominate the panel.
const SHORT_SCRIPT_MAX_LINES = 20;
const LONG_SCRIPT_HEIGHT = 400;

// Mirrors the error-message extraction used elsewhere in this codebase (e.g.
// appset-generated-apps-diff.tsx) for the same grpc-gateway JSON error envelope shape.
function extractErrorMessage(err: any): string {
    if (err?.response?.body?.message) {
        return err.response.body.message;
    }
    if (err?.response?.body?.error) {
        return err.response.body.error;
    }
    if (err?.message) {
        return err.message;
    }
    return 'unknown error';
}

// Presentational only: the resource health definition is fetched once, alongside the rest of the
// resource panel's data, by the DataLoader in resource-details.tsx so that
// switching to this tab is as instant as switching to any other (no per-visit network round trip).
export const ApplicationResourceHealthCheck = (props: {definition: models.ResourceHealthDefinition; error?: any}) => {
    const {definition, error} = props;

    // Report a failed fetch distinctly from a genuine "no health check" response - collapsing the
    // two would misrepresent an RBAC or network error as a fact about the resource.
    if (error) {
        return <p className='application-resource-health-check__message'>Unable to load the health check definition: {extractErrorMessage(error)}.</p>;
    }

    if (definition?.script) {
        const lineCount = definition.script.split('\n').length;
        const isLongScript = lineCount > SHORT_SCRIPT_MAX_LINES;
        return (
            <div className='white-box'>
                <div className='white-box__details'>
                    <p>{SOURCE_LABELS[definition.source] || definition.source} Lua health check script:</p>
                    <div className='application-resource-health-check__editor-wrapper'>
                        <MonacoEditor
                            vScrollBar={isLongScript}
                            minHeight={isLongScript ? LONG_SCRIPT_HEIGHT : 100}
                            maxHeight={isLongScript ? LONG_SCRIPT_HEIGHT : undefined}
                            scrollBeyondLastLine={false}
                            editor={{
                                input: {text: definition.script, language: 'lua'},
                                options: {readOnly: true, minimap: {enabled: false}, wordWrap: 'on', padding: {top: 10}}
                            }}
                        />
                    </div>
                </div>
            </div>
        );
    }
    if (definition?.source === 'built-in-go') {
        return (
            <p className='application-resource-health-check__message'>
                This resource kind's health is assessed by one of Argo CD's built-in checks (implemented in Go, not Lua), so there is no script to display here. See{' '}
                <a href='https://argo-cd.readthedocs.io/en/stable/operator-manual/health/' target='_blank' rel='noopener noreferrer'>
                    Resource Health
                </a>{' '}
                in the docs for what's checked for this kind.
            </p>
        );
    }
    return (
        <p className='application-resource-health-check__message'>
            No health check is defined for this resource kind. Argo CD considers it Healthy once it is successfully synced.
        </p>
    );
};
