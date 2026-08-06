import * as React from 'react';

import {Spinner} from '.';
import {ErrorBoundary} from './error-boundary/error-boundary';

export function lazyWithBoundary<P extends object>(Component: React.LazyExoticComponent<React.ComponentType<P>>, message: string): React.FC<P> {
    return (props: P) => (
        <ErrorBoundary message={message}>
            <React.Suspense fallback={<Spinner show={true} />}>
                <Component {...props} />
            </React.Suspense>
        </ErrorBoundary>
    );
}
