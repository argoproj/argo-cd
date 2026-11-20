import {Spinner} from './spinner';
import * as React from 'react';

// Suspends only the lazily loaded page, so the layout around it stays visible while a page chunk loads.
export const PageSuspense = ({children}: {children: React.ReactNode}) => (
    <React.Suspense
        fallback={
            <div style={{display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100vh'}}>
                <Spinner show={true} />
            </div>
        }>
        {children}
    </React.Suspense>
);
