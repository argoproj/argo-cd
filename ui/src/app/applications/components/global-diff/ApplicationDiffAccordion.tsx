import * as React from 'react';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ComparisonStatusIcon} from '../utils';
import {ApplicationResourcesDiff} from '../application-resources-diff/application-resources-diff';
import {Spinner} from '../../../shared/components';

export interface ApplicationDiffAccordionProps {
    appSummary: models.ApplicationDiffSummary;
    isLazy?: boolean;
    isLoadingLazy?: boolean;
    onExpand?: () => void;
    isOpen?: boolean;
    onToggle?: () => void;
}

export const ApplicationDiffAccordion = (props: ApplicationDiffAccordionProps) => {
    const {appSummary, isLazy, isLoadingLazy, onExpand, isOpen: controlledIsOpen, onToggle} = props;
    const [localIsOpen, setLocalIsOpen] = React.useState(true);
    const [isSyncing, setIsSyncing] = React.useState(false);

    const isOpen = controlledIsOpen !== undefined ? controlledIsOpen : localIsOpen;

    const handleHeaderClick = () => {
        if (onToggle) {
            onToggle();
        } else {
            const nextOpen = !localIsOpen;
            setLocalIsOpen(nextOpen);
            if (nextOpen && isLazy && onExpand) {
                onExpand();
            }
        }
    };

    const handleSync = async (e: React.MouseEvent) => {
        e.stopPropagation();
        setIsSyncing(true);
        try {
            await services.applications.sync(appSummary.appName, appSummary.appNamespace, null, false, false, null, null);
        } catch (err) {
            console.error('Failed to sync application', err);
        } finally {
            setIsSyncing(false);
        }
    };

    return (
        <div className={`application-diff-accordion ${isOpen ? 'application-diff-accordion--open' : ''}`}>
            <div className='application-diff-accordion__header' onClick={handleHeaderClick}>
                <div className='application-diff-accordion__title'>
                    <i className={`fas ${isOpen ? 'fa-chevron-down' : 'fa-chevron-right'} application-diff-accordion__icon`} />
                    <span className='application-diff-accordion__name'>{appSummary.appName}</span>
                    <span className='application-diff-accordion__project'>(Project: {appSummary.project})</span>
                </div>
                <div className='application-diff-accordion__badges'>
                    <ComparisonStatusIcon status={appSummary.syncStatus as models.SyncStatusCode} label={true} />
                    <span className='application-diff-accordion__diff-count badge badge--danger'>
                        {appSummary.diffs.length} drifted resource{appSummary.diffs.length !== 1 ? 's' : ''}
                    </span>
                    <button className='argo-button argo-button--base argo-button--sm' disabled={isSyncing} onClick={handleSync}>
                        {isSyncing ? 'Syncing...' : 'Sync'}
                    </button>
                </div>
            </div>
            {isOpen && (
                <div className='application-diff-accordion__content'>
                    {isLoadingLazy ? (
                        <div className='global-diff-loading'>
                            <Spinner show={true} />
                            <span>Loading detailed diff...</span>
                        </div>
                    ) : appSummary.diffs.length > 0 ? (
                        <ApplicationResourcesDiff states={appSummary.diffs} />
                    ) : (
                        <div className='application-diff-accordion__no-diff'>No manifest drift or out-of-sync resources found for this application.</div>
                    )}
                </div>
            )}
        </div>
    );
};
