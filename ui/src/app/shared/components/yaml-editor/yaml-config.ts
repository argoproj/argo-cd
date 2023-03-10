import {LanguageSettings} from 'monaco-kubernetes';

/**
 * The configuration of the editor.
 */
export type EditorConfig = {
    /**
     * Whether to use monaco-kubernetes in the editor.
     *
     * Enhancements
     *
     * - Schema validation for both well-known resources
     *   (e.g. Deployment, Ingress, etc) and Argo CRDs. This
     *   includes autocomplete and other common IntelliSense.
     * - Adds semantical validation which includes the most
     *   prevalent Kubernetes hardening guidelines and additional
     *   rules to prevent misconfigurations in Argo CD CRDs.
     *
     * @remark All heavy lifting is done in a web worker which
     *   leaves the main thread free for rendering the UI.
     */
    useKubernetesEditor: boolean;

    /**
     * Configuration of monaco-kubernetes.
     */

    settings?: LanguageSettings;
};

export const DEFAULT_EDITOR_CONFIG: EditorConfig = {
    useKubernetesEditor: true,
    settings: {
        // @see https://github.com/kubeshop/monokle-core/blob/main/packages/validation/docs/configuration.md
        validation: {
            plugins: {
                'yaml-syntax': true,
                'kubernetes-schema': true,
                'open-policy-agent': true,
                'argo': true
            },
            rules: {
                // @see https://github.com/kubeshop/monokle-core/blob/main/packages/validation/docs/core-plugins.md#open-policy-agent
                'open-policy-agent/no-latest-image': 'warn',
                'open-policy-agent/no-low-user-id': 'warn',
                'open-policy-agent/no-low-group-id': 'warn',
                'open-policy-agent/no-elevated-process': 'err',
                'open-policy-agent/no-sys-admin': 'err',
                'open-policy-agent/no-host-mounted-path': 'err',
                'open-policy-agent/no-host-port-access': 'err'
            }
        }
    }
};
