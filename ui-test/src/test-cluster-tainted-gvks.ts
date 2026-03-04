import Configuration from './Configuration';
import UiTestUtilities from './UiTestUtilities';
import {trace} from 'console';
import {SettingsPage} from './settings/settings-page';
import {ClusterDetailsPage} from './settings/cluster-details-page';

/**
 * Test that validates the new tainted GVK fields on the cluster details page:
 * - DEGRADED RESOURCES COUNT: Shows the count of failed resource GVKs
 * - DEGRADED RESOURCES: Shows the list of failed resource GVKs
 *
 * These fields should only be displayed when there are conversion webhook failures
 * that result in tainted/failed resource GVKs in the cluster cache.
 *
 * This test verifies:
 * 1. Navigation to cluster details page works
 * 2. Basic cluster information is displayed
 * 3. Degraded resources fields are properly handled (either shown with data or hidden when no issues)
 * 4. Field values are properly formatted and valid
 */
async function testClusterTaintedGVKFields() {
    const navigation = await UiTestUtilities.init();
    try {
        if (Configuration.ARGOCD_AUTH_USERNAME !== '') {
            await navigation.getLoginPage().loginWithCredentials();
        }

        UiTestUtilities.log('Starting cluster tainted GVK fields test');

        // Navigate to Settings > Clusters
        const settingsPage: SettingsPage = await navigation.clickSettingsNavBarButton();
        await settingsPage.clickClustersTab();
        UiTestUtilities.log('Successfully navigated to clusters page');

        // Click on the first available cluster (or target cluster if specified)
        const clusterDetailsPage: ClusterDetailsPage = await settingsPage.clickFirstCluster();
        await clusterDetailsPage.waitForPageLoad();
        UiTestUtilities.log('Successfully loaded cluster details page');

        // Verify basic cluster information is displayed
        const resourcesCount = await clusterDetailsPage.getResourcesCount();
        const applicationsCount = await clusterDetailsPage.getApplicationsCount();

        UiTestUtilities.log(`Cluster resources count: ${resourcesCount}`);
        UiTestUtilities.log(`Cluster applications count: ${applicationsCount}`);

        // Check if resources count is a valid number
        const resourcesNumber = parseInt(resourcesCount, 10);
        if (isNaN(resourcesNumber)) {
            throw new Error(`Invalid resources count format: ${resourcesCount}`);
        }

        // Check if applications count is a valid number
        const applicationsNumber = parseInt(applicationsCount, 10);
        if (isNaN(applicationsNumber)) {
            throw new Error(`Invalid applications count format: ${applicationsCount}`);
        }

        UiTestUtilities.log('Basic cluster information validation passed');

        // Test degraded resources fields
        const isDegradedCountPresent = await clusterDetailsPage.isDegradedResourcesCountPresent();
        const isDegradedListPresent = await clusterDetailsPage.isDegradedResourcesListPresent();

        if (isDegradedCountPresent && isDegradedListPresent) {
            // Both fields are present - validate they contain proper data
            UiTestUtilities.log('Degraded resources fields are present - validating content');

            const degradedInfo = await clusterDetailsPage.verifyDegradedResourcesFieldsWhenPresent();

            // Additional validation: count should match the number of items in the list
            const listItems = degradedInfo.list
                .split(',')
                .map((item) => item.trim())
                .filter((item) => item.length > 0);
            const countNumber = parseInt(degradedInfo.count, 10);

            if (listItems.length !== countNumber) {
                UiTestUtilities.log(`Warning: Count (${countNumber}) doesn't match list items (${listItems.length}). This might be expected if list is truncated.`);
            }

            // Validate that list items look like GVKs (should contain dots or slashes)
            for (const item of listItems) {
                if (!item.includes('.') && !item.includes('/')) {
                    UiTestUtilities.log(`Warning: GVK item '${item}' doesn't appear to be in expected format`);
                }
            }

            UiTestUtilities.log('✅ Degraded resources fields validation passed - fields are present with valid data');
        } else if (!isDegradedCountPresent && !isDegradedListPresent) {
            // Neither field is present - this is expected when there are no failed GVKs
            await clusterDetailsPage.verifyDegradedResourcesFieldsWhenAbsent();
            UiTestUtilities.log('✅ Degraded resources fields validation passed - fields are correctly hidden when no issues exist');
        } else {
            // Only one field is present - this is inconsistent
            throw new Error(`Inconsistent degraded resources fields state: count present=${isDegradedCountPresent}, list present=${isDegradedListPresent}`);
        }

        UiTestUtilities.log('🎉 Cluster tainted GVK fields test completed successfully');
    } catch (e) {
        trace('❌ Cluster tainted GVK fields test failed: ', e);
        throw e;
    } finally {
        await navigation.quit();
    }
}

// Run the test
testClusterTaintedGVKFields()
    .then(() => {
        UiTestUtilities.log('Test execution completed');
    })
    .catch((error) => {
        UiTestUtilities.logError(`Test execution failed: ${error}`);
        process.exit(1);
    });
