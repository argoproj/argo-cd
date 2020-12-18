const ARGO_SUCCESS_COLOR = '#18BE94';
const ARGO_WARNING_COLOR = '#f4c030';
const ARGO_FAILED_COLOR = '#E96D76';
const ARGO_RUNNING_COLOR = '#0DADEA';
const ARGO_GRAY4_COLOR = '#CCD6DD';

export const COLORS = {
    connection_status: {
        failed: ARGO_FAILED_COLOR,
        successful: ARGO_SUCCESS_COLOR,
        unknown: ARGO_GRAY4_COLOR
    },
    health: {
        degraded: ARGO_FAILED_COLOR,
        healthy: ARGO_SUCCESS_COLOR,
        missing: ARGO_GRAY4_COLOR,
        progressing: ARGO_RUNNING_COLOR,
        suspended: ARGO_GRAY4_COLOR,
        unknown: ARGO_GRAY4_COLOR
    },
    operation: {
        error: ARGO_FAILED_COLOR,
        failed: ARGO_FAILED_COLOR,
        running: ARGO_RUNNING_COLOR,
        success: ARGO_SUCCESS_COLOR,
        terminating: ARGO_RUNNING_COLOR
    },
    sync: {
        synced: ARGO_SUCCESS_COLOR,
        out_of_sync: ARGO_WARNING_COLOR,
        unknown: ARGO_GRAY4_COLOR
    },
    sync_result: {
        failed: ARGO_FAILED_COLOR,
        synced: ARGO_SUCCESS_COLOR,
        pruned: ARGO_GRAY4_COLOR,
        unknown: ARGO_GRAY4_COLOR
    },
    sync_window: {
        deny: ARGO_FAILED_COLOR,
        allow: ARGO_SUCCESS_COLOR,
        manual: ARGO_WARNING_COLOR,
        inactive: ARGO_GRAY4_COLOR,
        unknown: ARGO_GRAY4_COLOR
    }
};
