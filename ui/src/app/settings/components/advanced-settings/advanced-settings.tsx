import {Tabs} from 'argo-ui';
import * as React from 'react';

import {AuthOption, EditablePanel, Page, Query} from '../../../shared/components';
import {EditablePanelItem} from '../../../shared/components/editable-panel/editable-panel';
import {AuthSettingsCtx, Context} from '../../../shared/context';
import {AuthSettings} from '../../../shared/models';
import {MonacoEditor} from '../../../shared/components/monaco-editor';

require('./advanced-settings.scss');

const NOT_CONFIGURED = '';

type AdvancedItem = EditablePanelItem & {configured: boolean};

// A boolean is considered configured whenever it is present in the payload, even when false.
const boolItem = (title: string, value?: boolean): AdvancedItem => ({
    title,
    view: value ? 'true' : 'false',
    configured: value !== undefined && value !== null
});

// A text/list value is considered configured only when it is non-empty.
const textItem = (title: string, value?: string): AdvancedItem => ({
    title,
    view: value || NOT_CONFIGURED,
    configured: !!value
});

export const AdvancedSettings = () => {
    const authSettings = React.useContext(AuthSettingsCtx);
    const ctx = React.useContext(Context);
    const [showAll, setShowAll] = React.useState(false);

    const renderPanel = (title: string, settings: AuthSettings, allItems: AdvancedItem[]) => {
        const configuredItems = allItems.filter(item => item.configured);
        const unconfiguredItems = allItems.filter(item => !item.configured);
        const itemsToShow = showAll ? [...configuredItems, ...unconfiguredItems] : configuredItems;

        if (itemsToShow.length === 0) {
            return null;
        }

        return <EditablePanel title={title} values={settings} noReadonlyMode={true} items={itemsToShow} />;
    };

    const configurationTab = (settings: AuthSettings) => {
        const generalItems: AdvancedItem[] = [
            textItem('App Label Key', settings.appLabelKey),
            textItem('Tracking Method', settings.trackingMethod),
            textItem('Additional URLs', settings.additionalUrls && settings.additionalUrls.length > 0 ? settings.additionalUrls.join(', ') : ''),
            boolItem('Status Badge Enabled', settings.statusBadgeEnabled),
            textItem('Status Badge Root URL', settings.statusBadgeRootUrl),
            textItem('UI CSS URL', settings.uiCssURL),
            textItem('UI Banner Content', settings.uiBannerContent),
            textItem('UI Banner URL', settings.uiBannerURL),
            textItem('UI Banner Position', settings.uiBannerPosition),
            boolItem('UI Banner Permanent', settings.uiBannerPermanent)
        ];

        const instanceItems: AdvancedItem[] = [textItem('Controller Namespace', settings.controllerNamespace), textItem('Installation ID', settings.installationID)];

        const authenticationItems: AdvancedItem[] = [
            boolItem('User Logins Disabled', settings.userLoginsDisabled),
            textItem('Dex Connectors', settings.dexConfig?.connectors ? settings.dexConfig.connectors.map(c => `${c.name} (${c.type})`).join(', ') : ''),
            textItem('OIDC Provider', settings.oidcConfig?.name)
        ];

        const featuresItems: AdvancedItem[] = [
            boolItem('Exec Enabled', settings.execEnabled),
            boolItem('Apps in Any Namespace Enabled', settings.appsInAnyNamespaceEnabled),
            boolItem('Hydrator Enabled', settings.hydratorEnabled),
            boolItem('Sync with Replace Allowed', settings.syncWithReplaceAllowed),
            boolItem('Impersonation Enabled', settings.impersonationEnabled),
            boolItem('Resource View Enabled', settings.resourceViewEnabled)
        ];

        const helpAnalyticsItems: AdvancedItem[] = [
            textItem('Google Analytics Tracking ID', settings.googleAnalytics?.trackingID),
            boolItem('Google Analytics Anonymize Users', settings.googleAnalytics?.anonymizeUsers),
            textItem('Help Chat URL', settings.help?.chatUrl),
            textItem('Help Chat Text', settings.help?.chatText),
            textItem(
                'Binary URLs',
                settings.help?.binaryUrls
                    ? Object.entries(settings.help.binaryUrls)
                          .map(([k, v]) => `${k}: ${v}`)
                          .join(', ')
                    : ''
            )
        ];

        const kustomizeItems: AdvancedItem[] = [
            textItem('Kustomize Versions', settings.kustomizeVersions && settings.kustomizeVersions.length > 0 ? settings.kustomizeVersions.join(', ') : '')
        ];

        return (
            <div className='argo-container'>
                <div className='advanced-settings__panels'>
                    {renderPanel('GENERAL', settings, generalItems)}
                    {renderPanel('FEATURES', settings, featuresItems)}
                    {renderPanel('INSTANCE', settings, instanceItems)}
                    {renderPanel('AUTHENTICATION', settings, authenticationItems)}
                    {renderPanel('HELP & ANALYTICS', settings, helpAnalyticsItems)}
                    {renderPanel('KUSTOMIZE', settings, kustomizeItems)}
                </div>
            </div>
        );
    };

    const jsonTab = (settings: AuthSettings) => {
        return (
            <div className='argo-container advanced-settings__json'>
                <MonacoEditor
                    minHeight={600}
                    vScrollBar={true}
                    editor={{
                        input: {
                            text: JSON.stringify(settings, null, 2),
                            language: 'json'
                        },
                        options: {
                            readOnly: true,
                            minimap: {enabled: false}
                        }
                    }}
                />
            </div>
        );
    };

    return (
        <Query>
            {params => {
                const selectedTab = params.get('tab') || 'configuration';
                return (
                    <Page
                        title='Advanced'
                        toolbar={{
                            breadcrumbs: [{title: 'Settings', path: '/settings'}, {title: 'Advanced'}],
                            actionMenu:
                                selectedTab === 'configuration'
                                    ? {
                                          items: [
                                              {
                                                  title: showAll ? 'Show Configured Only' : 'Show All Options',
                                                  iconClassName: 'fa fa-filter',
                                                  action: () => setShowAll(prev => !prev)
                                              }
                                          ]
                                      }
                                    : undefined,
                            tools: <AuthOption />
                        }}>
                        <Tabs
                            selectedTabKey={selectedTab}
                            onTabSelected={tab => ctx.navigation.goto('.', {tab}, {replace: true})}
                            navCenter={true}
                            tabs={[
                                {
                                    key: 'configuration',
                                    title: 'Configuration',
                                    content: configurationTab(authSettings)
                                },
                                {
                                    key: 'json',
                                    title: 'JSON',
                                    content: jsonTab(authSettings)
                                }
                            ].map(tab => ({...tab, isOnlyContentScrollable: true, extraVerticalScrollPadding: 160}))}
                        />
                    </Page>
                );
            }}
        </Query>
    );
};
