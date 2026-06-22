import {Tabs} from 'argo-ui';
import * as React from 'react';

import {AuthOption, EditablePanel, Page, Query} from '../../../shared/components';
import {EditablePanelItem} from '../../../shared/components/editable-panel/editable-panel';
import {AuthSettingsCtx, Context} from '../../../shared/context';
import {AuthSettings} from '../../../shared/models';
import {MonacoEditor} from '../../../shared/components/monaco-editor';

require('./advanced-settings.scss');

const NOT_CONFIGURED = '';

const isConfiguredItem = (item: EditablePanelItem): boolean => {
    const viewString = typeof item.view === 'string' ? item.view : String(item.view);
    return viewString !== NOT_CONFIGURED && viewString !== 'false';
};

export const AdvancedSettings = () => {
    const authSettings = React.useContext(AuthSettingsCtx);
    const ctx = React.useContext(Context);
    const [showAll, setShowAll] = React.useState(false);

    const renderPanel = (title: string, settings: AuthSettings, allItems: EditablePanelItem[]) => {
        const configuredItems = allItems.filter(isConfiguredItem);
        const unconfiguredItems = allItems.filter(item => !isConfiguredItem(item));
        const itemsToShow = showAll ? [...configuredItems, ...unconfiguredItems] : configuredItems;

        if (itemsToShow.length === 0) {
            return null;
        }

        return <EditablePanel title={title} values={settings} noReadonlyMode={true} items={itemsToShow} />;
    };

    const configurationTab = (settings: AuthSettings) => {
        const generalItems: EditablePanelItem[] = [
            {
                title: 'Status Badge Enabled',
                view: settings.statusBadgeEnabled ? 'true' : 'false'
            },
            {
                title: 'Status Badge Root URL',
                view: settings.statusBadgeRootUrl || NOT_CONFIGURED
            },
            {
                title: 'UI CSS URL',
                view: settings.uiCssURL || NOT_CONFIGURED
            },
            {
                title: 'UI Banner Content',
                view: settings.uiBannerContent || NOT_CONFIGURED
            },
            {
                title: 'UI Banner URL',
                view: settings.uiBannerURL || NOT_CONFIGURED
            },
            {
                title: 'UI Banner Position',
                view: settings.uiBannerPosition || NOT_CONFIGURED
            },
            {
                title: 'UI Banner Permanent',
                view: settings.uiBannerPermanent ? 'true' : 'false'
            }
        ];

        const authenticationItems: EditablePanelItem[] = [
            {
                title: 'User Logins Disabled',
                view: settings.userLoginsDisabled ? 'true' : 'false'
            },
            {
                title: 'Dex Connectors',
                view: settings.dexConfig?.connectors ? settings.dexConfig.connectors.map(c => `${c.name} (${c.type})`).join(', ') : NOT_CONFIGURED
            },
            {
                title: 'OIDC Provider',
                view: settings.oidcConfig?.name || NOT_CONFIGURED
            }
        ];

        const featuresItems: EditablePanelItem[] = [
            {
                title: 'Exec Enabled',
                view: settings.execEnabled ? 'true' : 'false'
            },
            {
                title: 'Apps in Any Namespace Enabled',
                view: settings.appsInAnyNamespaceEnabled ? 'true' : 'false'
            },
            {
                title: 'Hydrator Enabled',
                view: settings.hydratorEnabled ? 'true' : 'false'
            },
            {
                title: 'Sync with Replace Allowed',
                view: settings.syncWithReplaceAllowed ? 'true' : 'false'
            }
        ];

        const helpAnalyticsItems: EditablePanelItem[] = [
            {
                title: 'Google Analytics Tracking ID',
                view: settings.googleAnalytics?.trackingID || NOT_CONFIGURED
            },
            {
                title: 'Google Analytics Anonymize Users',
                view: settings.googleAnalytics?.anonymizeUsers ? 'true' : 'false'
            },
            {
                title: 'Help Chat URL',
                view: settings.help?.chatUrl || NOT_CONFIGURED
            },
            {
                title: 'Help Chat Text',
                view: settings.help?.chatText || NOT_CONFIGURED
            },
            {
                title: 'Binary URLs',
                view: settings.help?.binaryUrls
                    ? Object.entries(settings.help.binaryUrls)
                          .map(([k, v]) => `${k}: ${v}`)
                          .join(', ')
                    : NOT_CONFIGURED
            }
        ];

        const kustomizeItems: EditablePanelItem[] = [
            {
                title: 'Kustomize Versions',
                view: settings.kustomizeVersions && settings.kustomizeVersions.length > 0 ? settings.kustomizeVersions.join(', ') : NOT_CONFIGURED
            }
        ];

        return (
            <div className='argo-container'>
                <div className='advanced-settings__panels'>
                    {renderPanel('GENERAL', settings, generalItems)}
                    {renderPanel('AUTHENTICATION', settings, authenticationItems)}
                    {renderPanel('FEATURES', settings, featuresItems)}
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
