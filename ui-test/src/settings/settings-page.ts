import {By, WebDriver} from 'selenium-webdriver';
import {Base} from '../base';
import UiTestUtilities from '../UiTestUtilities';
import {ClusterDetailsPage} from './cluster-details-page';

const CLUSTERS_TAB: By = By.css('a[href="/settings/clusters"]');
const CLUSTER_ROW_SELECTOR: By = By.css('.argo-table-list__row');

export class SettingsPage extends Base {
    private clusterDetailsPage: ClusterDetailsPage;

    public constructor(driver: WebDriver) {
        super(driver);
        this.clusterDetailsPage = new ClusterDetailsPage(this.driver);
    }

    /**
     * Click the Clusters tab in settings
     */
    public async clickClustersTab(): Promise<void> {
        try {
            const clustersTab = await UiTestUtilities.findUiElement(this.driver, CLUSTERS_TAB);
            await clustersTab.click();
        } catch (err: any) {
            throw new Error(`Failed to click clusters tab: ${err}`);
        }
    }

    /**
     * Click on a cluster by name to view its details
     * @param clusterName - The name of the cluster to click
     */
    public async clickClusterByName(clusterName: string): Promise<ClusterDetailsPage> {
        try {
            // Find all cluster rows
            const clusterRows = await this.driver.findElements(CLUSTER_ROW_SELECTOR);

            for (const row of clusterRows) {
                const rowText = await row.getText();
                if (rowText.includes(clusterName)) {
                    await row.click();
                    return this.clusterDetailsPage;
                }
            }

            throw new Error(`Cluster '${clusterName}' not found in clusters list`);
        } catch (err: any) {
            throw new Error(`Failed to click cluster '${clusterName}': ${err}`);
        }
    }

    /**
     * Get the first available cluster (useful for testing)
     */
    public async clickFirstCluster(): Promise<ClusterDetailsPage> {
        try {
            const clusterRows = await this.driver.findElements(CLUSTER_ROW_SELECTOR);
            if (clusterRows.length === 0) {
                throw new Error('No clusters found');
            }

            await clusterRows[0].click();
            return this.clusterDetailsPage;
        } catch (err: any) {
            throw new Error(`Failed to click first cluster: ${err}`);
        }
    }
}
